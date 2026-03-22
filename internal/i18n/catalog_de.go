package i18n

func init() {
	register(LocaleDE, map[string]string{
		// Common validation
		MsgRequired:         "%s ist erforderlich",
		MsgInvalidID:        "ungültige %s-ID",
		MsgNotFound:         "%s nicht gefunden: %s",
		MsgAlreadyExists:    "%s existiert bereits: %s",
		MsgInvalidRequest:   "ungültige Anfrage: %s",
		MsgInvalidJSON:      "ungültiges JSON",
		MsgUnauthorized:     "nicht autorisiert",
		MsgPermissionDenied: "Zugriff verweigert: unzureichende Rolle für %s",
		MsgInternalError:    "interner Fehler: %s",
		MsgInvalidSlug:      "%s muss ein gültiger Slug sein (nur Kleinbuchstaben, Zahlen und Bindestriche)",
		MsgFailedToList:     "Auflisten von %s fehlgeschlagen",
		MsgFailedToCreate:   "Erstellen von %s fehlgeschlagen: %s",
		MsgFailedToUpdate:   "Aktualisieren von %s fehlgeschlagen: %s",
		MsgFailedToDelete:   "Löschen von %s fehlgeschlagen: %s",
		MsgFailedToSave:     "Speichern von %s fehlgeschlagen: %s",
		MsgInvalidUpdates:   "ungültige Aktualisierungen",

		// Agent
		MsgAgentNotFound:       "Agent nicht gefunden: %s",
		MsgCannotDeleteDefault: "der Standard-Agent kann nicht gelöscht werden",
		MsgUserCtxRequired:     "Benutzerkontext erforderlich",

		// Chat
		MsgRateLimitExceeded: "Anfragelimit überschritten — bitte warten",
		MsgNoUserMessage:     "keine Benutzernachricht gefunden",
		MsgUserIDRequired:    "user_id ist erforderlich",
		MsgMsgRequired:       "Nachricht ist erforderlich",

		// Channel instances
		MsgInvalidChannelType: "ungültiger channel_type",
		MsgInstanceNotFound:   "Instanz nicht gefunden",

		// Cron
		MsgJobNotFound:     "Job nicht gefunden",
		MsgInvalidCronExpr: "ungültiger cron-Ausdruck: %s",

		// Config
		MsgConfigHashMismatch: "die Konfiguration hat sich geändert (Hash stimmt nicht überein)",

		// Exec approval
		MsgExecApprovalDisabled: "Ausführungsgenehmigung ist nicht aktiviert",

		// Pairing
		MsgSenderChannelRequired: "senderId und channel sind erforderlich",
		MsgCodeRequired:          "Code ist erforderlich",
		MsgSenderIDRequired:      "sender_id ist erforderlich",

		// HTTP API
		MsgInvalidAuth:           "ungültige Authentifizierung",
		MsgMsgsRequired:          "messages ist erforderlich",
		MsgUserIDHeader:          "X-ArgoClaw-User-Id-Header ist erforderlich",
		MsgFileTooLarge:          "Datei zu groß oder ungültiges Multipart-Formular",
		MsgMissingFileField:      "Feld 'file' fehlt",
		MsgInvalidFilename:       "ungültiger Dateiname",
		MsgChannelKeyReq:         "channel und key sind erforderlich",
		MsgMethodNotAllowed:      "Methode nicht erlaubt",
		MsgStreamingNotSupported: "Streaming nicht unterstützt",
		MsgOwnerOnly:             "nur der Eigentümer kann %s",
		MsgNoAccess:              "kein Zugriff auf dieses %s",
		MsgAlreadySummoning:      "der Agent wird bereits herbeigerufen",
		MsgSummoningUnavailable:  "Herbeirufung nicht verfügbar",
		MsgNoDescription:         "der Agent hat keine Beschreibung für eine erneute Herbeirufung",
		MsgInvalidPath:           "ungültiger Pfad",

		// Scheduler
		MsgQueueFull:    "Sitzungswarteschlange ist voll",
		MsgShuttingDown: "das Gateway wird heruntergefahren, bitte kurz erneut versuchen",

		// Provider
		MsgProviderReqFailed: "%s: Anfrage fehlgeschlagen: %s",

		// Unknown method
		MsgUnknownMethod: "unbekannte Methode: %s",

		// Not implemented
		MsgNotImplemented: "%s noch nicht implementiert",

		// Agent links
		MsgLinksNotConfigured:   "Agent-Links nicht konfiguriert",
		MsgInvalidDirection:     "die Richtung muss outbound, inbound oder bidirectional sein",
		MsgSourceTargetSame:     "Quelle und Ziel müssen verschiedene Agenten sein",
		MsgCannotDelegateOpen:   "Delegation an offene Agenten nicht möglich — nur vordefinierte Agenten können Delegationsziele sein",
		MsgNoUpdatesProvided:    "keine Aktualisierungen angegeben",
		MsgInvalidLinkStatus:    "der Status muss active oder disabled sein",

		// Teams
		MsgTeamsNotConfigured:   "Teams nicht konfiguriert",
		MsgAgentIsTeamLead:      "der Agent ist bereits der Teamleiter",
		MsgCannotRemoveTeamLead: "der Teamleiter kann nicht entfernt werden",

		// Delegations
		MsgDelegationsUnavailable: "Delegationen nicht verfügbar",

		// Channels
		MsgCannotDeleteDefaultInst: "die Standard-Kanalinstanz kann nicht gelöscht werden",

		// Skills
		MsgSkillsUpdateNotSupported: "skills.update wird für dateibasierte Skills nicht unterstützt",
		MsgCannotResolveSkillID:     "die Skill-ID für dateibasierte Skills kann nicht aufgelöst werden",

		// Logs
		MsgInvalidLogAction: "die Aktion muss 'start' oder 'stop' sein",

		// Config
		MsgRawConfigRequired: "Raw-Konfiguration ist erforderlich",
		MsgRawPatchRequired:  "Raw-Patch ist erforderlich",

		// Storage / File
		MsgCannotDeleteSkillsDir: "Skill-Verzeichnisse können nicht gelöscht werden",
		MsgFailedToReadFile:      "Datei konnte nicht gelesen werden",
		MsgFileNotFound:          "Datei nicht gefunden",
		MsgInvalidVersion:        "ungültige Version",
		MsgVersionNotFound:       "Version nicht gefunden",
		MsgFailedToDeleteFile:    "Löschen fehlgeschlagen",

		// OAuth
		MsgNoPendingOAuth:    "kein ausstehender OAuth-Ablauf",
		MsgFailedToSaveToken: "Token konnte nicht gespeichert werden",

		// Status
		MsgStatusWorking:       "🔄 Ich arbeite an Ihrer Anfrage... Bitte warten.",
		MsgStatusDetailed:      "🔄 Ich arbeite gerade an Ihrer Anfrage...\n%s (Iteration %d)\nLäuft seit: %s\n\nBitte warten — ich antworte, wenn ich fertig bin.",
		MsgStatusPhaseThinking: "Phase: Nachdenken...",
		MsgStatusPhaseToolExec: "Phase: Ausführung von %s",
		MsgStatusPhaseTools:    "Phase: Werkzeuge ausführen...",
		MsgStatusPhaseCompact:  "Phase: Kontext komprimieren...",
		MsgStatusPhaseDefault:  "Phase: Verarbeitung...",
		MsgCancelledReply:      "✋ Abgebrochen. Was möchten Sie als Nächstes tun?",
		MsgInjectedAck:         "Verstanden, ich werde das in meine aktuelle Arbeit einbeziehen.",

		// Knowledge Graph
		MsgEntityIDRequired:       "entity_id ist erforderlich",
		MsgEntityFieldsRequired:   "external_id, name und entity_type sind erforderlich",
		MsgTextRequired:           "text ist erforderlich",
		MsgProviderModelRequired:  "provider und model sind erforderlich",
		MsgInvalidProviderOrModel: "ungültiger provider oder model",

		// Builtin tool descriptions
		MsgToolReadFile:        "Den Inhalt einer Datei aus dem Workspace des Agenten anhand des Pfads lesen",
		MsgToolWriteFile:       "Inhalt in eine Datei im Workspace schreiben und Verzeichnisse bei Bedarf erstellen",
		MsgToolListFiles:       "Dateien und Verzeichnisse in einem Pfad innerhalb des Workspace auflisten",
		MsgToolEdit:            "Gezielte Such-und-Ersetz-Änderungen an bestehenden Dateien anwenden, ohne die gesamte Datei neu zu schreiben",
		MsgToolExec:            "Einen Shell-Befehl im Workspace ausführen und stdout/stderr zurückgeben",
		MsgToolWebSearch:       "Das Web nach Informationen durchsuchen mit einer Suchmaschine (Brave oder DuckDuckGo)",
		MsgToolWebFetch:        "Eine Webseite oder einen API-Endpoint abrufen und den Textinhalt extrahieren",
		MsgToolMemorySearch:    "Das Langzeitgedächtnis des Agenten mittels semantischer Ähnlichkeit durchsuchen",
		MsgToolMemoryGet:       "Ein bestimmtes Gedächtnisdokument anhand seines Dateipfads abrufen",
		MsgToolKGSearch:        "Entitäten, Beziehungen und Beobachtungen im Wissensgraphen des Agenten suchen",
		MsgToolReadImage:       "Bilder analysieren mit einem visionsfähigen LLM-Anbieter",
		MsgToolReadDocument:    "Dokumente (PDF, Word, Excel, PowerPoint, CSV usw.) analysieren mit einem dokumentenfähigen LLM-Anbieter",
		MsgToolCreateImage:     "Bilder aus Textprompts mit einem Bildgenerierungsanbieter erzeugen",
		MsgToolReadAudio:       "Audiodateien (Sprache, Musik, Geräusche) analysieren mit einem audiofähigen LLM-Anbieter",
		MsgToolReadVideo:       "Videodateien analysieren mit einem videofähigen LLM-Anbieter",
		MsgToolCreateVideo:     "Videos aus Textbeschreibungen mit KI generieren",
		MsgToolCreateAudio:     "Musik oder Soundeffekte aus Textbeschreibungen mit KI generieren",
		MsgToolTTS:             "Text in natürlich klingende Sprachaudiodateien umwandeln",
		MsgToolBrowser:         "Browser-Interaktionen automatisieren: Seiten navigieren, Elemente anklicken, Formulare ausfüllen, Screenshots aufnehmen",
		MsgToolSessionsList:    "Aktive Chat-Sitzungen über alle Kanäle auflisten",
		MsgToolSessionStatus:   "Den aktuellen Status und die Metadaten einer bestimmten Chat-Sitzung abrufen",
		MsgToolSessionsHistory: "Den Nachrichtenverlauf einer bestimmten Chat-Sitzung abrufen",
		MsgToolSessionsSend:    "Eine Nachricht an eine aktive Chat-Sitzung im Namen des Agenten senden",
		MsgToolMessage:         "Eine proaktive Nachricht an einen Benutzer auf einem verbundenen Kanal senden (Telegram, Discord usw.)",
		MsgToolCron:            "Wiederkehrende Aufgaben mit cron-Ausdrücken, Zeitangaben oder Intervallen planen oder verwalten",
		MsgToolSpawn:           "Einen Sub-Agenten für Hintergrundarbeit erstellen oder eine Aufgabe an einen verknüpften Agenten delegieren",
		MsgToolSkillSearch:     "Verfügbare Skills nach Schlüsselwort oder Beschreibung durchsuchen, um relevante Fähigkeiten zu finden",
		MsgToolUseSkill:        "Einen Skill aktivieren, um seine spezialisierten Fähigkeiten zu nutzen (Tracing-Marker)",
		MsgToolSkillManage:     "Skills aus der Konversationserfahrung erstellen, bearbeiten oder löschen",
		MsgToolPublishSkill:    "Ein Skill-Verzeichnis in der Systemdatenbank registrieren und auffindbar machen",
		MsgToolTeamTasks:       "Aufgaben auf dem Team-Aufgabenboard anzeigen, erstellen, aktualisieren und abschließen",

		MsgSkillNudgePostscript: "Diese Aufgabe umfasste mehrere Schritte. Soll ich den Prozess als wiederverwendbaren Skill speichern? Antworten Sie **\"als Skill speichern\"** oder **\"überspringen\"**.",
		MsgSkillNudge70Pct:      "[System] Sie sind bei 70% Ihres Iterationsbudgets. Überlegen Sie, ob Muster aus dieser Sitzung einen guten Skill ergeben würden.",
		MsgSkillNudge90Pct:      "[System] Sie sind bei 90% Ihres Iterationsbudgets. Wenn diese Sitzung wiederverwendbare Muster enthielt, erwägen Sie, diese als Skill zu speichern, bevor Sie abschließen.",
	})
}
