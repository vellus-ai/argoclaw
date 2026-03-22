package i18n

func init() {
	register(LocaleFR, map[string]string{
		// Common validation
		MsgRequired:         "%s est requis",
		MsgInvalidID:        "ID de %s invalide",
		MsgNotFound:         "%s introuvable : %s",
		MsgAlreadyExists:    "%s existe déjà : %s",
		MsgInvalidRequest:   "requête invalide : %s",
		MsgInvalidJSON:      "JSON invalide",
		MsgUnauthorized:     "non autorisé",
		MsgPermissionDenied: "permission refusée : rôle insuffisant pour %s",
		MsgInternalError:    "erreur interne : %s",
		MsgInvalidSlug:      "%s doit être un slug valide (lettres minuscules, chiffres et tirets uniquement)",
		MsgFailedToList:     "échec de la liste de %s",
		MsgFailedToCreate:   "échec de la création de %s : %s",
		MsgFailedToUpdate:   "échec de la mise à jour de %s : %s",
		MsgFailedToDelete:   "échec de la suppression de %s : %s",
		MsgFailedToSave:     "échec de la sauvegarde de %s : %s",
		MsgInvalidUpdates:   "mises à jour invalides",

		// Agent
		MsgAgentNotFound:       "agent introuvable : %s",
		MsgCannotDeleteDefault: "impossible de supprimer l'agent par défaut",
		MsgUserCtxRequired:     "contexte utilisateur requis",

		// Chat
		MsgRateLimitExceeded: "limite de requêtes dépassée — veuillez patienter",
		MsgNoUserMessage:     "aucun message utilisateur trouvé",
		MsgUserIDRequired:    "user_id est requis",
		MsgMsgRequired:       "le message est requis",

		// Channel instances
		MsgInvalidChannelType: "channel_type invalide",
		MsgInstanceNotFound:   "instance introuvable",

		// Cron
		MsgJobNotFound:     "job introuvable",
		MsgInvalidCronExpr: "expression cron invalide : %s",

		// Config
		MsgConfigHashMismatch: "la configuration a changé (hash différent)",

		// Exec approval
		MsgExecApprovalDisabled: "l'approbation d'exécution n'est pas activée",

		// Pairing
		MsgSenderChannelRequired: "senderId et channel sont requis",
		MsgCodeRequired:          "le code est requis",
		MsgSenderIDRequired:      "sender_id est requis",

		// HTTP API
		MsgInvalidAuth:           "authentification invalide",
		MsgMsgsRequired:          "messages est requis",
		MsgUserIDHeader:          "l'en-tête X-ArgoClaw-User-Id est requis",
		MsgFileTooLarge:          "fichier trop volumineux ou formulaire multipart invalide",
		MsgMissingFileField:      "champ 'file' manquant",
		MsgInvalidFilename:       "nom de fichier invalide",
		MsgChannelKeyReq:         "channel et key sont requis",
		MsgMethodNotAllowed:      "méthode non autorisée",
		MsgStreamingNotSupported: "streaming non pris en charge",
		MsgOwnerOnly:             "seul le propriétaire peut %s",
		MsgNoAccess:              "aucun accès à ce %s",
		MsgAlreadySummoning:      "l'agent est déjà en cours d'invocation",
		MsgSummoningUnavailable:  "invocation non disponible",
		MsgNoDescription:         "l'agent n'a pas de description pour la réinvocation",
		MsgInvalidPath:           "chemin invalide",

		// Scheduler
		MsgQueueFull:    "la file d'attente de session est pleine",
		MsgShuttingDown: "le gateway est en cours d'arrêt, veuillez réessayer sous peu",

		// Provider
		MsgProviderReqFailed: "%s : requête échouée : %s",

		// Unknown method
		MsgUnknownMethod: "méthode inconnue : %s",

		// Not implemented
		MsgNotImplemented: "%s pas encore implémenté",

		// Agent links
		MsgLinksNotConfigured:   "liens de l'agent non configurés",
		MsgInvalidDirection:     "la direction doit être outbound, inbound ou bidirectional",
		MsgSourceTargetSame:     "la source et la cible doivent être des agents différents",
		MsgCannotDelegateOpen:   "impossible de déléguer à des agents ouverts — seuls les agents prédéfinis peuvent être des cibles de délégation",
		MsgNoUpdatesProvided:    "aucune mise à jour fournie",
		MsgInvalidLinkStatus:    "le statut doit être active ou disabled",

		// Teams
		MsgTeamsNotConfigured:   "équipes non configurées",
		MsgAgentIsTeamLead:      "l'agent est déjà le chef d'équipe",
		MsgCannotRemoveTeamLead: "impossible de retirer le chef d'équipe",

		// Delegations
		MsgDelegationsUnavailable: "délégations non disponibles",

		// Channels
		MsgCannotDeleteDefaultInst: "impossible de supprimer l'instance de canal par défaut",

		// Skills
		MsgSkillsUpdateNotSupported: "skills.update non pris en charge pour les skills basées sur fichier",
		MsgCannotResolveSkillID:     "impossible de résoudre l'ID de la skill basée sur fichier",

		// Logs
		MsgInvalidLogAction: "l'action doit être 'start' ou 'stop'",

		// Config
		MsgRawConfigRequired: "la configuration raw est requise",
		MsgRawPatchRequired:  "le patch raw est requis",

		// Storage / File
		MsgCannotDeleteSkillsDir: "impossible de supprimer les répertoires de skills",
		MsgFailedToReadFile:      "échec de la lecture du fichier",
		MsgFileNotFound:          "fichier introuvable",
		MsgInvalidVersion:        "version invalide",
		MsgVersionNotFound:       "version introuvable",
		MsgFailedToDeleteFile:    "échec de la suppression",

		// OAuth
		MsgNoPendingOAuth:    "aucun flux OAuth en attente",
		MsgFailedToSaveToken: "échec de la sauvegarde du token",

		// Status
		MsgStatusWorking:       "🔄 Je travaille sur votre demande... Veuillez patienter.",
		MsgStatusDetailed:      "🔄 Je travaille actuellement sur votre demande...\n%s (itération %d)\nEn cours depuis : %s\n\nVeuillez patienter — je répondrai une fois terminé.",
		MsgStatusPhaseThinking: "Phase : Réflexion...",
		MsgStatusPhaseToolExec: "Phase : Exécution de %s",
		MsgStatusPhaseTools:    "Phase : Exécution des outils...",
		MsgStatusPhaseCompact:  "Phase : Compactage du contexte...",
		MsgStatusPhaseDefault:  "Phase : Traitement...",
		MsgCancelledReply:      "✋ Annulé. Que souhaitez-vous faire ensuite ?",
		MsgInjectedAck:         "Compris, je vais intégrer cela dans ce sur quoi je travaille.",

		// Knowledge Graph
		MsgEntityIDRequired:       "entity_id est requis",
		MsgEntityFieldsRequired:   "external_id, name et entity_type sont requis",
		MsgTextRequired:           "text est requis",
		MsgProviderModelRequired:  "provider et model sont requis",
		MsgInvalidProviderOrModel: "provider ou model invalide",

		// Builtin tool descriptions
		MsgToolReadFile:        "Lire le contenu d'un fichier du workspace de l'agent par son chemin",
		MsgToolWriteFile:       "Écrire du contenu dans un fichier du workspace, en créant les répertoires si nécessaire",
		MsgToolListFiles:       "Lister les fichiers et répertoires dans un chemin du workspace",
		MsgToolEdit:            "Appliquer des modifications ciblées de recherche et remplacement sur des fichiers existants sans réécrire le fichier entier",
		MsgToolExec:            "Exécuter une commande shell dans le workspace et retourner stdout/stderr",
		MsgToolWebSearch:       "Rechercher des informations sur le web à l'aide d'un moteur de recherche (Brave ou DuckDuckGo)",
		MsgToolWebFetch:        "Récupérer une page web ou un endpoint API et extraire son contenu textuel",
		MsgToolMemorySearch:    "Rechercher dans la mémoire à long terme de l'agent par similarité sémantique",
		MsgToolMemoryGet:       "Récupérer un document mémoire spécifique par son chemin de fichier",
		MsgToolKGSearch:        "Rechercher des entités, relations et observations dans le graphe de connaissances de l'agent",
		MsgToolReadImage:       "Analyser des images à l'aide d'un fournisseur LLM avec capacité de vision",
		MsgToolReadDocument:    "Analyser des documents (PDF, Word, Excel, PowerPoint, CSV, etc.) à l'aide d'un fournisseur LLM avec capacité documentaire",
		MsgToolCreateImage:     "Générer des images à partir de prompts textuels à l'aide d'un fournisseur de génération d'images",
		MsgToolReadAudio:       "Analyser des fichiers audio (parole, musique, sons) à l'aide d'un fournisseur LLM avec capacité audio",
		MsgToolReadVideo:       "Analyser des fichiers vidéo à l'aide d'un fournisseur LLM avec capacité vidéo",
		MsgToolCreateVideo:     "Générer des vidéos à partir de descriptions textuelles par IA",
		MsgToolCreateAudio:     "Générer de la musique ou des effets sonores à partir de descriptions textuelles par IA",
		MsgToolTTS:             "Convertir du texte en audio vocal au son naturel",
		MsgToolBrowser:         "Automatiser les interactions navigateur : naviguer sur les pages, cliquer sur les éléments, remplir les formulaires, capturer des écrans",
		MsgToolSessionsList:    "Lister les sessions de chat actives sur tous les canaux",
		MsgToolSessionStatus:   "Obtenir l'état actuel et les métadonnées d'une session de chat spécifique",
		MsgToolSessionsHistory: "Récupérer l'historique des messages d'une session de chat spécifique",
		MsgToolSessionsSend:    "Envoyer un message à une session de chat active au nom de l'agent",
		MsgToolMessage:         "Envoyer un message proactif à un utilisateur sur un canal connecté (Telegram, Discord, etc.)",
		MsgToolCron:            "Planifier ou gérer des tâches récurrentes à l'aide d'expressions cron, d'horaires ou d'intervalles",
		MsgToolSpawn:           "Créer un sous-agent pour un travail en arrière-plan ou déléguer une tâche à un agent lié",
		MsgToolSkillSearch:     "Rechercher des skills disponibles par mot-clé ou description pour trouver des capacités pertinentes",
		MsgToolUseSkill:        "Activer une skill pour utiliser ses capacités spécialisées (marqueur de traçage)",
		MsgToolSkillManage:     "Créer, modifier ou supprimer des skills depuis l'expérience de conversation",
		MsgToolPublishSkill:    "Enregistrer un répertoire de skill dans la base de données système, le rendant découvrable",
		MsgToolTeamTasks:       "Voir, créer, mettre à jour et compléter des tâches sur le tableau des tâches de l'équipe",

		MsgSkillNudgePostscript: "Cette tâche a impliqué plusieurs étapes. Voulez-vous que je sauvegarde le processus comme skill réutilisable ? Répondez **\"sauvegarder comme skill\"** ou **\"passer\"**.",
		MsgSkillNudge70Pct:      "[Système] Vous êtes à 70% de votre budget d'itérations. Réfléchissez si des modèles de cette session feraient une bonne skill.",
		MsgSkillNudge90Pct:      "[Système] Vous êtes à 90% de votre budget d'itérations. Si cette session a impliqué des modèles réutilisables, envisagez de les sauvegarder comme skill avant de terminer.",
	})
}
