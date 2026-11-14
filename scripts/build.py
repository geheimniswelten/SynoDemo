#!/usr/bin/env python3
"""Build an unsigned, reproducible DSM 7.1+ package for x86-64."""
from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import struct
import subprocess
import sys
import tarfile

ROOT = Path(__file__).resolve().parents[1]
PACKAGE = ROOT / "package"
SIZES = (16, 24, 32, 48, 64, 72, 128, 256)


def read_text_file(path: Path) -> bytes:
    return path.read_bytes().replace(b"\r\n", b"\n")


def tar_bytes(files: dict[str, tuple[bytes, int]]) -> bytes:
    output = io.BytesIO()
    parents = set()
    for name in files:
        path = PurePosixPath(name)
        if path.is_absolute() or ".." in path.parts:
            raise ValueError(f"Unsafe archive path: {name}")
        parents.update(p.as_posix() for p in path.parents if p != PurePosixPath("."))
    with tarfile.open(fileobj=output, mode="w", format=tarfile.USTAR_FORMAT) as archive:
        for name in sorted(parents):
            entry = tarfile.TarInfo(name)
            entry.type = tarfile.DIRTYPE
            entry.mode = 0o755
            entry.uname = entry.gname = "root"
            archive.addfile(entry)
        for name, (content, mode) in sorted(files.items()):
            entry = tarfile.TarInfo(name)
            entry.size = len(content)
            entry.mode = mode
            entry.uname = entry.gname = "root"
            archive.addfile(entry, io.BytesIO(content))
    return output.getvalue()


TARGETS = {
    "x86_64": ("amd64", 62, 2),
    "rtd1296": ("arm64", 183, 2),
    "alpine": ("arm", 40, 1),
    "alpine4k": ("arm", 40, 1),
}

def verify_elf(binary: bytes, target: str) -> None:
    _, machine, elfclass = TARGETS[target]
    if len(binary) < 64 or binary[:4] != b"\x7fELF" or binary[4] != elfclass or binary[5] != 1 or struct.unpack_from("<H", binary, 18)[0] != machine:
        raise ValueError("Backend ELF architecture does not match target")
    if elfclass == 2:
        header_offset = struct.unpack_from("<Q", binary, 32)[0]
        header_size, header_count = struct.unpack_from("<HH", binary, 54)
        minimum = 56
    else:
        header_offset = struct.unpack_from("<I", binary, 28)[0]
        header_size, header_count = struct.unpack_from("<HH", binary, 42)
        minimum = 32
    if header_size < minimum or header_count < 1 or header_offset + header_size * header_count > len(binary):
        raise ValueError("Invalid backend ELF program headers")
    for number in range(header_count):
        if struct.unpack_from("<I", binary, header_offset + number * header_size)[0] in (2, 3):
            raise ValueError("Backend must be statically linked")


def compile_backend(go: str, target: str) -> bytes:
    output = ROOT / "build" / ("synodemo-" + target)
    output.parent.mkdir(parents=True, exist_ok=True)
    environment = dict(os.environ, GOOS="linux", GOARCH=TARGETS[target][0], GOAMD64="v1", GOARM="7", CGO_ENABLED="0", GOTOOLCHAIN="local")
    subprocess.run([go, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", str(output), "./cmd/synodemo"], cwd=ROOT, env=environment, check=True)
    return output.read_bytes()


def icon(path: Path, size: int) -> bytes:
    data = path.read_bytes()
    magic = b"\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"
    if len(data) < 33 or not data.startswith(magic) or struct.unpack_from(">II", data, 16) != (size, size):
        raise ValueError(f"Expected a {size} x {size} PNG: {path.relative_to(ROOT)}")
    return data


def info_fields(info: str) -> dict[str, str]:
    fields: dict[str, str] = {}
    for line in info.splitlines():
        found = re.fullmatch(r'([a-zA-Z_]+)="([^"]*)"', line)
        if not found or found[1] in fields:
            raise ValueError(f"Invalid or duplicate package metadata: {line}")
        fields[found[1]] = found[2]
    if fields.get("package") != "h5uSynoDemo" or fields.get("arch") not in TARGETS:
        raise ValueError("INFO package identity or architecture differs from the backend")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+-[0-9]+", fields.get("version", "")):
        raise ValueError("Missing or invalid version in INFO")
    if "checksum" in fields:
        raise ValueError("The builder adds the package checksum to INFO")
    for key in ("dsmappname", "dsmuidir", "os_min_ver"):
        if not fields.get(key):
            raise ValueError(f"Missing package metadata: {key}")
    return fields


def prepare_ui(payload: dict[str, tuple[bytes, int]], metadata: dict[str, str]) -> None:
    ui = PACKAGE / "ui"
    if (ui / "style.css").exists():
        raise ValueError("DSM reserves ui/style.css for the outer desktop")
    for filename in ("index.html", "config", "SynoDemo.js"):
        if not (ui / filename).is_file():
            raise ValueError(f"Missing UI file: {filename}")
    config = json.loads(read_text_file(ui / "config"))
    if config.get("SynoDemo.js", {}).get(metadata["dsmappname"], {}).get("type") != "app":
        raise ValueError("INFO and the DSM application manifest disagree")
    for path in sorted(ui.rglob("*")):
        if path.is_symlink():
            raise ValueError(f"Package UI must not contain symlinks: {path}")
        if not path.is_file():
            continue
        name = path.relative_to(PACKAGE).as_posix()
        if name == "ui/api.cgi":
            raise ValueError("api.cgi is produced by the package build")
        if path.suffix.lower() == ".png":
            data = path.read_bytes()
        elif path.name == "config" or path.suffix.lower() in {".html", ".js", ".css", ".json", ".svg"}:
            data = read_text_file(path)
        else:
            raise ValueError(f"Unexpected UI asset: {name}")
        if name == "ui/index.html":
            if data.count(b"@PACKAGE_VERSION@") != 1:
                raise ValueError("Missing or duplicate package version placeholder")
            data = data.replace(b"@PACKAGE_VERSION@", metadata["version"].encode("ascii"))
        if path.name == "SynoDemo.js":
            # Invalidate cached HTML even when rebuilding the same fixed version.
            ui_revision = hashlib.sha256(read_text_file(ui / "index.html")).hexdigest()[:12]
            data = data.replace(b"/index.html", f"/index.html?v={metadata['version']}-{ui_revision}".encode("ascii"))
        payload[name] = (data, 0o644)
    for size in SIZES:
        path = ui / "images" / f"icon_{size}.png"
        icon(path, size)


def build(binary: bytes, destination: Path | None, version: str, target: str) -> Path:
    verify_elf(binary, target)
    info = read_text_file(PACKAGE / "INFO").decode("utf-8").strip()
    info = re.sub(r'(?m)^arch="[^"]+"$', f'arch="{target}"', info)
    info = re.sub(r'(?m)^version="[^"]+"$', f'version="{version}"', info)
    metadata = info_fields(info)
    privilege = read_text_file(PACKAGE / "conf" / "privilege")
    if json.loads(privilege) != {"defaults": {"run-as": "package"}}:
        raise ValueError("The service must run as the unprivileged package account")
    payload = {
        "bin/synodemo": (binary, 0o755),
        "ui/api.cgi": (binary, 0o755),
        "run/.keep": (b"", 0o644),
    }
    prepare_ui(payload, metadata)
    buffer = io.BytesIO()
    with gzip.GzipFile(fileobj=buffer, mode="wb", filename="", mtime=0) as archive:
        archive.write(tar_bytes(payload))
    compressed = buffer.getvalue()
    checksum = hashlib.md5(compressed, usedforsecurity=False).hexdigest()
    outer = {
        "INFO": ((info + f'\nchecksum="{checksum}"\n').encode("utf-8"), 0o644),
        "PACKAGE_ICON.PNG": (icon(PACKAGE / "PACKAGE_ICON.PNG", 64), 0o644),
        "PACKAGE_ICON_256.PNG": (icon(PACKAGE / "PACKAGE_ICON_256.PNG", 256), 0o644),
        "conf/privilege": (privilege, 0o644),
        "package.tgz": (compressed, 0o644),
    }
    for name in ("preinst", "postinst", "preuninst", "postuninst", "preupgrade", "postupgrade", "start-stop-status"):
        content = read_text_file(PACKAGE / "scripts" / name)
        if not content.startswith(b"#!/bin/sh\n"):
            raise ValueError(f"Invalid lifecycle script: {name}")
        outer[f"scripts/{name}"] = (content, 0o755)

    output = destination or ROOT / "dist" / f"{metadata['package']}-{metadata['version']}-{metadata['arch']}.spk"
    output.parent.mkdir(parents=True, exist_ok=True)
    data = tar_bytes(outer)
    output.write_bytes(data)
    digest = hashlib.sha256(data).hexdigest()
    output.with_suffix(output.suffix + ".sha256").write_text(f"{digest}  {output.name}\n", encoding="ascii", newline="\n")
    print(f"SPK: {output}")
    print(f"SHA256: {digest}")
    print(f"Target: {metadata['arch']} / DSM >= {metadata['os_min_ver']}")
    return output


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--arch", choices=tuple(TARGETS), default="x86_64")
    parser.add_argument("--version", choices=("1.0.0-0000", "1.2.0-0000"), default="1.0.0-0000")
    parser.add_argument("--go", default="go", help="Path to the Go executable")
    parser.add_argument("--binary", type=Path, help="Package an existing Linux/amd64 executable")
    parser.add_argument("--output", type=Path, help="Path for the completed SPK")
    args = parser.parse_args()
    try:
        build(args.binary.read_bytes() if args.binary else compile_backend(args.go, args.arch), args.output, args.version, args.arch)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Build failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

