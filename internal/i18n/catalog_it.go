package i18n

func init() {
	register(LocaleIT, map[string]string{
		// Common validation
		MsgRequired:         "%s è obbligatorio",
		MsgInvalidID:        "ID di %s non valido",
		MsgNotFound:         "%s non trovato: %s",
		MsgAlreadyExists:    "%s esiste già: %s",
		MsgInvalidRequest:   "richiesta non valida: %s",
		MsgInvalidJSON:      "JSON non valido",
		MsgUnauthorized:     "non autorizzato",
		MsgPermissionDenied: "permesso negato: ruolo insufficiente per %s",
		MsgInternalError:    "errore interno: %s",
		MsgInvalidSlug:      "%s deve essere un slug valido (solo lettere minuscole, numeri e trattini)",
		MsgFailedToList:     "impossibile elencare %s",
		MsgFailedToCreate:   "impossibile creare %s: %s",
		MsgFailedToUpdate:   "impossibile aggiornare %s: %s",
		MsgFailedToDelete:   "impossibile eliminare %s: %s",
		MsgFailedToSave:     "impossibile salvare %s: %s",
		MsgInvalidUpdates:   "aggiornamenti non validi",

		// Agent
		MsgAgentNotFound:       "agente non trovato: %s",
		MsgCannotDeleteDefault: "impossibile eliminare l'agente predefinito",
		MsgUserCtxRequired:     "contesto utente richiesto",

		// Chat
		MsgRateLimitExceeded: "limite di richieste superato — attendere prego",
		MsgNoUserMessage:     "nessun messaggio utente trovato",
		MsgUserIDRequired:    "user_id è obbligatorio",
		MsgMsgRequired:       "il messaggio è obbligatorio",

		// Channel instances
		MsgInvalidChannelType: "channel_type non valido",
		MsgInstanceNotFound:   "istanza non trovata",

		// Cron
		MsgJobNotFound:     "job non trovato",
		MsgInvalidCronExpr: "espressione cron non valida: %s",

		// Config
		MsgConfigHashMismatch: "la configurazione è cambiata (hash non corrispondente)",

		// Exec approval
		MsgExecApprovalDisabled: "l'approvazione dell'esecuzione non è abilitata",

		// Pairing
		MsgSenderChannelRequired: "senderId e channel sono obbligatori",
		MsgCodeRequired:          "il codice è obbligatorio",
		MsgSenderIDRequired:      "sender_id è obbligatorio",

		// HTTP API
		MsgInvalidAuth:           "autenticazione non valida",
		MsgMsgsRequired:          "messages è obbligatorio",
		MsgUserIDHeader:          "l'intestazione X-ArgoClaw-User-Id è obbligatoria",
		MsgFileTooLarge:          "file troppo grande o modulo multipart non valido",
		MsgMissingFileField:      "campo 'file' mancante",
		MsgInvalidFilename:       "nome file non valido",
		MsgChannelKeyReq:         "channel e key sono obbligatori",
		MsgMethodNotAllowed:      "metodo non consentito",
		MsgStreamingNotSupported: "streaming non supportato",
		MsgOwnerOnly:             "solo il proprietario può %s",
		MsgNoAccess:              "nessun accesso a questo %s",
		MsgAlreadySummoning:      "l'agente è già in fase di invocazione",
		MsgSummoningUnavailable:  "invocazione non disponibile",
		MsgNoDescription:         "l'agente non ha una descrizione per la reinvocazione",
		MsgInvalidPath:           "percorso non valido",

		// Scheduler
		MsgQueueFull:    "la coda della sessione è piena",
		MsgShuttingDown: "il gateway si sta arrestando, riprovare a breve",

		// Provider
		MsgProviderReqFailed: "%s: richiesta fallita: %s",

		// Unknown method
		MsgUnknownMethod: "metodo sconosciuto: %s",

		// Not implemented
		MsgNotImplemented: "%s non ancora implementato",

		// Agent links
		MsgLinksNotConfigured:   "collegamenti dell'agente non configurati",
		MsgInvalidDirection:     "la direzione deve essere outbound, inbound o bidirectional",
		MsgSourceTargetSame:     "origine e destinazione devono essere agenti diversi",
		MsgCannotDelegateOpen:   "impossibile delegare ad agenti aperti — solo gli agenti predefiniti possono essere obiettivi di delega",
		MsgNoUpdatesProvided:    "nessun aggiornamento fornito",
		MsgInvalidLinkStatus:    "lo stato deve essere active o disabled",

		// Teams
		MsgTeamsNotConfigured:   "team non configurati",
		MsgAgentIsTeamLead:      "l'agente è già il capo del team",
		MsgCannotRemoveTeamLead: "impossibile rimuovere il capo del team",

		// Delegations
		MsgDelegationsUnavailable: "deleghe non disponibili",

		// Channels
		MsgCannotDeleteDefaultInst: "impossibile eliminare l'istanza di canale predefinita",

		// Skills
		MsgSkillsUpdateNotSupported: "skills.update non supportato per le skill basate su file",
		MsgCannotResolveSkillID:     "impossibile risolvere l'ID della skill basata su file",

		// Logs
		MsgInvalidLogAction: "l'azione deve essere 'start' o 'stop'",

		// Config
		MsgRawConfigRequired: "la configurazione raw è obbligatoria",
		MsgRawPatchRequired:  "il patch raw è obbligatorio",

		// Storage / File
		MsgCannotDeleteSkillsDir: "impossibile eliminare le directory delle skill",
		MsgFailedToReadFile:      "impossibile leggere il file",
		MsgFileNotFound:          "file non trovato",
		MsgInvalidVersion:        "versione non valida",
		MsgVersionNotFound:       "versione non trovata",
		MsgFailedToDeleteFile:    "impossibile eliminare",

		// OAuth
		MsgNoPendingOAuth:    "nessun flusso OAuth in attesa",
		MsgFailedToSaveToken: "impossibile salvare il token",

		// Status
		MsgStatusWorking:       "🔄 Sto lavorando alla tua richiesta... Attendere prego.",
		MsgStatusDetailed:      "🔄 Sto attualmente lavorando alla tua richiesta...\n%s (iterazione %d)\nIn esecuzione da: %s\n\nAttendere prego — risponderò quando avrò finito.",
		MsgStatusPhaseThinking: "Fase: Riflessione...",
		MsgStatusPhaseToolExec: "Fase: Esecuzione di %s",
		MsgStatusPhaseTools:    "Fase: Esecuzione degli strumenti...",
		MsgStatusPhaseCompact:  "Fase: Compattazione del contesto...",
		MsgStatusPhaseDefault:  "Fase: Elaborazione...",
		MsgCancelledReply:      "✋ Annullato. Cosa vorresti fare ora?",
		MsgInjectedAck:         "Capito, lo integrerò in quello su cui sto lavorando.",

		// Knowledge Graph
		MsgEntityIDRequired:       "entity_id è obbligatorio",
		MsgEntityFieldsRequired:   "external_id, name ed entity_type sono obbligatori",
		MsgTextRequired:           "text è obbligatorio",
		MsgProviderModelRequired:  "provider e model sono obbligatori",
		MsgInvalidProviderOrModel: "provider o model non valido",

		// Builtin tool descriptions
		MsgToolReadFile:        "Leggere il contenuto di un file dal workspace dell'agente tramite percorso",
		MsgToolWriteFile:       "Scrivere contenuto in un file nel workspace, creando le directory se necessario",
		MsgToolListFiles:       "Elencare file e directory in un percorso all'interno del workspace",
		MsgToolEdit:            "Applicare modifiche mirate di ricerca e sostituzione su file esistenti senza riscrivere l'intero file",
		MsgToolExec:            "Eseguire un comando shell nel workspace e restituire stdout/stderr",
		MsgToolWebSearch:       "Cercare informazioni sul web utilizzando un motore di ricerca (Brave o DuckDuckGo)",
		MsgToolWebFetch:        "Recuperare una pagina web o un endpoint API ed estrarne il contenuto testuale",
		MsgToolMemorySearch:    "Cercare nella memoria a lungo termine dell'agente tramite similarità semantica",
		MsgToolMemoryGet:       "Recuperare un documento di memoria specifico tramite il percorso del file",
		MsgToolKGSearch:        "Cercare entità, relazioni e osservazioni nel grafo della conoscenza dell'agente",
		MsgToolReadImage:       "Analizzare immagini utilizzando un fornitore LLM con capacità di visione",
		MsgToolReadDocument:    "Analizzare documenti (PDF, Word, Excel, PowerPoint, CSV, ecc.) utilizzando un fornitore LLM con capacità documentale",
		MsgToolCreateImage:     "Generare immagini da prompt testuali utilizzando un fornitore di generazione immagini",
		MsgToolReadAudio:       "Analizzare file audio (parlato, musica, suoni) utilizzando un fornitore LLM con capacità audio",
		MsgToolReadVideo:       "Analizzare file video utilizzando un fornitore LLM con capacità video",
		MsgToolCreateVideo:     "Generare video da descrizioni testuali utilizzando l'IA",
		MsgToolCreateAudio:     "Generare musica o effetti sonori da descrizioni testuali utilizzando l'IA",
		MsgToolTTS:             "Convertire testo in audio vocale dal suono naturale",
		MsgToolBrowser:         "Automatizzare le interazioni del browser: navigare pagine, cliccare elementi, compilare moduli, catturare schermate",
		MsgToolSessionsList:    "Elencare le sessioni di chat attive su tutti i canali",
		MsgToolSessionStatus:   "Ottenere lo stato attuale e i metadati di una sessione di chat specifica",
		MsgToolSessionsHistory: "Recuperare la cronologia dei messaggi di una sessione di chat specifica",
		MsgToolSessionsSend:    "Inviare un messaggio a una sessione di chat attiva per conto dell'agente",
		MsgToolMessage:         "Inviare un messaggio proattivo a un utente su un canale connesso (Telegram, Discord, ecc.)",
		MsgToolCron:            "Pianificare o gestire attività ricorrenti tramite espressioni cron, orari o intervalli",
		MsgToolSpawn:           "Creare un sotto-agente per lavoro in background o delegare un compito a un agente collegato",
		MsgToolSkillSearch:     "Cercare skill disponibili per parola chiave o descrizione per trovare capacità pertinenti",
		MsgToolUseSkill:        "Attivare una skill per utilizzare le sue capacità specializzate (marcatore di tracciamento)",
		MsgToolSkillManage:     "Creare, modificare o eliminare skill dall'esperienza di conversazione",
		MsgToolPublishSkill:    "Registrare una directory di skill nel database di sistema, rendendola scopribile",
		MsgToolTeamTasks:       "Visualizzare, creare, aggiornare e completare attività nella bacheca delle attività del team",

		MsgSkillNudgePostscript: "Questa attività ha comportato diversi passaggi. Vuoi che salvi il processo come skill riutilizzabile? Rispondi **\"salva come skill\"** o **\"salta\"**.",
		MsgSkillNudge70Pct:      "[Sistema] Sei al 70% del tuo budget di iterazioni. Valuta se qualche schema di questa sessione potrebbe diventare una buona skill.",
		MsgSkillNudge90Pct:      "[Sistema] Sei al 90% del tuo budget di iterazioni. Se questa sessione ha coinvolto schemi riutilizzabili, considera di salvarli come skill prima di concludere.",
	})
}
