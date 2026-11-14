@echo off
rem Projektbezogene Einstellungen fuer RELEASE.cmd
set "REPO=geheimniswelten/SynoDemo"
set "REMOTE=origin"
set "TITLE_TEMPLATE=SynoDemo {VERSION}"
set "NOTES_DEFAULT=Fehlerkorrekturen und Verbesserungen."
set "ASSET_MASK=dist\h5uSynoDemo-{VERSION}-*.spk*"
set "VERSION_FILE=package\INFO"
set "VERSION_FORMAT=info"
set "VERSION_TRIM_BUILD=1"
set "TAG_PREFIX=v"
