# Installation und Paketquelle

Diese Anleitung ist ein Entwurf; die Schritte müssen noch mit den jeweiligen DSM-Versionen geprüft und um Screenshots ergänzt werden.

## Manuelle Installation

1. Das zu NAS-Architektur und DSM-Version passende SPK herunterladen. Für diesen Prototyp: DSM 7.1 oder neuer; `x86_64` für DS918+ und `rtd1296` für DS218, sowie `alpine`/`alpine4k` für die jeweiligen ARMv7-Plattformen.
2. Im **Paket-Zentrum** **Manuelle Installation** wählen und das SPK auswählen. Den angezeigten Hinweisen zu einem nicht signierten Drittanbieterpaket folgen.
3. Nach der Installation SynoDemo im DSM-Hauptmenü öffnen. Zum Entfernen im Paket-Zentrum das Paket auswählen und **Deinstallieren** wählen.

## Eigene Paketquelle

1. Die Paketquelle nach der README in `synology-packages` auf einem erreichbaren PHP-Webserver einrichten. Deren vollständige URL wird benötigt; der lokale Windows-Pfad ist keine DSM-Paketquellen-URL.
2. Im Paket-Zentrum unter **Einstellungen → Paketquellen** eine Quelle hinzufügen und die URL eintragen. Die genaue Bezeichnung kann je nach DSM-Version abweichen.
3. Nach Aktualisierung der Paketliste nach dem Paket suchen und es installieren.

Die hier verwendete Paketquelle liest öffentliche GitHub-Releases. Damit SynoDemo erscheint, muss es dort konfiguriert sein und ein passendes Release mit SPK sowie eine zum Tag passende `package/INFO` besitzen. Für einen Updatetest zuerst Version 1.0.0 veröffentlichen und installieren, danach Version 1.2.0 mit passender INFO veröffentlichen. Automatische Updates hängen von den DSM-Einstellungen und dem Aktualisierungsintervall der Quelle ab. Bis zur tatsächlichen Veröffentlichung und Erprobung ist dies kein bestätigter Ablauf.
