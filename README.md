# SynoDemo

**Technik:** Die DSM-Oberfläche verwendet HTML, CSS und JavaScript; das Backend und der Paketdienst sind in Go geschrieben. Go wird nur zum Bauen benötigt: Das SPK enthält eine statisch gebaute Linux-Programmdatei. Die DSM-Integration verwendet außerdem Shellskripte.

**Weitere Möglichkeiten:** DSM-Pakete können auch andere native Programme, etwa aus C, C++ oder Rust, sowie Skripte z. B. in Python, PHP oder Shell enthalten. Je nach Sprache muss die passende Laufzeit oder Bibliothek vorhanden sein oder mitgeliefert werden. Eine klassische GTK- oder Qt-Oberfläche benötigt eine gesonderte grafische Umgebung; die übliche DSM-Oberfläche ist webbasiert.

**Zielarchitekturen:** x86_64 (64-Bit Intel/AMD, z. B. DS918+), ARMv8 / aarch64 (64-Bit ARM, z. B. DS218 / `rtd1296`) und ARMv7 (32-Bit ARM, `alpine` und `alpine4k`). Die Pakete sind getrennt gebaut und bislang auf keinem dieser Geräte getestet.

**Arbeitsstand, noch nicht auf DSM getestet.** SynoDemo ist eine kleine zweisprachige Demo für die Einbindung eines Pakets in DSM. Der aktuelle Stand enthält Übersicht, Protokoll, Einstellungen und einen abschaltbaren periodischen Ereignisgenerator im Paketdienst. Er zielt auf DSM 7.1 und neuer; ein Administratorkonto ist erforderlich.

Die Integration des DSM-Aufgabenplaners einschließlich normaler und Root-Aufgaben, deren Entfernung bei Deinstallation sowie DSM-Protokoll, Benachrichtigungen und E-Mail ist noch nicht implementiert. Dieses Paket ist daher noch keine vollständige Demonstration der vorgesehenen Funktionen.

## Build

`BUILD.cmd` erzeugt beide unveränderlichen Testversionen `1.0.0-0000` und `1.2.0-0000` für x86_64, rtd1296, alpine und alpine4k im Verzeichnis `dist`. Mit `-Version 1.0.0-0000` oder `-Version 1.2.0-0000` wird nur eine Version erzeugt. `-Architecture rtd1296` begrenzt den Build beispielsweise auf die DS218. Go 1.24+ und Python 3.10+ werden benötigt; Werkzeugpfade können mit `-Go` und `-Python` gesetzt werden. Das Versionssuffix entspricht dem Format der vorhandenen Pakete. Die Versionsnummer wird beim Build nicht erhöht.

Die SPKs unterscheiden sich bislang nur in den Paketmetadaten und der Cache-Version des DSM-Fensters. Ein tatsächlicher Installations- und Upgradetest auf DSM steht aus. Sie sind unsigniert.

Icons werden bei Bedarf mit `python scripts/generate_icons.py` neu erzeugt.

[Installationshinweise](.github/INSTALL.md) · [Hinweise zum Aufgabenplaner](.github/TASKS.md)
