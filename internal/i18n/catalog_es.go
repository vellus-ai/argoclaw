package i18n

func init() {
	register(LocaleES, map[string]string{
		// Common validation
		MsgRequired:         "%s es obligatorio",
		MsgInvalidID:        "ID de %s no válido",
		MsgNotFound:         "%s no encontrado: %s",
		MsgAlreadyExists:    "%s ya existe: %s",
		MsgInvalidRequest:   "solicitud no válida: %s",
		MsgInvalidJSON:      "JSON no válido",
		MsgUnauthorized:     "no autorizado",
		MsgPermissionDenied: "permiso denegado: rol insuficiente para %s",
		MsgInternalError:    "error interno: %s",
		MsgInvalidSlug:      "%s debe ser un slug válido (solo letras minúsculas, números y guiones)",
		MsgFailedToList:     "error al listar %s",
		MsgFailedToCreate:   "error al crear %s: %s",
		MsgFailedToUpdate:   "error al actualizar %s: %s",
		MsgFailedToDelete:   "error al eliminar %s: %s",
		MsgFailedToSave:     "error al guardar %s: %s",
		MsgInvalidUpdates:   "actualizaciones no válidas",

		// Agent
		MsgAgentNotFound:       "agente no encontrado: %s",
		MsgCannotDeleteDefault: "no se puede eliminar el agente predeterminado",
		MsgUserCtxRequired:     "se requiere contexto de usuario",

		// Chat
		MsgRateLimitExceeded: "límite de solicitudes excedido — por favor espere",
		MsgNoUserMessage:     "no se encontró mensaje del usuario",
		MsgUserIDRequired:    "user_id es obligatorio",
		MsgMsgRequired:       "el mensaje es obligatorio",

		// Channel instances
		MsgInvalidChannelType: "channel_type no válido",
		MsgInstanceNotFound:   "instancia no encontrada",

		// Cron
		MsgJobNotFound:     "job no encontrado",
		MsgInvalidCronExpr: "expresión cron no válida: %s",

		// Config
		MsgConfigHashMismatch: "la configuración ha cambiado (hash no coincide)",

		// Exec approval
		MsgExecApprovalDisabled: "la aprobación de ejecución no está habilitada",

		// Pairing
		MsgSenderChannelRequired: "senderId y channel son obligatorios",
		MsgCodeRequired:          "el código es obligatorio",
		MsgSenderIDRequired:      "sender_id es obligatorio",

		// HTTP API
		MsgInvalidAuth:           "autenticación no válida",
		MsgMsgsRequired:          "messages es obligatorio",
		MsgUserIDHeader:          "el encabezado X-ArgoClaw-User-Id es obligatorio",
		MsgFileTooLarge:          "archivo demasiado grande o formulario multipart no válido",
		MsgMissingFileField:      "falta el campo 'file'",
		MsgInvalidFilename:       "nombre de archivo no válido",
		MsgChannelKeyReq:         "channel y key son obligatorios",
		MsgMethodNotAllowed:      "método no permitido",
		MsgStreamingNotSupported: "streaming no soportado",
		MsgOwnerOnly:             "solo el propietario puede %s",
		MsgNoAccess:              "sin acceso a este %s",
		MsgAlreadySummoning:      "el agente ya está siendo invocado",
		MsgSummoningUnavailable:  "invocación no disponible",
		MsgNoDescription:         "el agente no tiene descripción para reinvocar",
		MsgInvalidPath:           "ruta no válida",

		// Scheduler
		MsgQueueFull:    "la cola de sesión está llena",
		MsgShuttingDown: "el gateway se está cerrando, reintente en breve",

		// Provider
		MsgProviderReqFailed: "%s: solicitud fallida: %s",

		// Unknown method
		MsgUnknownMethod: "método desconocido: %s",

		// Not implemented
		MsgNotImplemented: "%s aún no implementado",

		// Agent links
		MsgLinksNotConfigured:   "enlaces del agente no configurados",
		MsgInvalidDirection:     "la dirección debe ser outbound, inbound o bidirectional",
		MsgSourceTargetSame:     "el origen y el destino deben ser agentes diferentes",
		MsgCannotDelegateOpen:   "no se puede delegar a agentes abiertos — solo los agentes predefinidos pueden ser objetivos de delegación",
		MsgNoUpdatesProvided:    "no se proporcionaron actualizaciones",
		MsgInvalidLinkStatus:    "el estado debe ser active o disabled",

		// Teams
		MsgTeamsNotConfigured:   "equipos no configurados",
		MsgAgentIsTeamLead:      "el agente ya es el líder del equipo",
		MsgCannotRemoveTeamLead: "no se puede eliminar al líder del equipo",

		// Delegations
		MsgDelegationsUnavailable: "delegaciones no disponibles",

		// Channels
		MsgCannotDeleteDefaultInst: "no se puede eliminar la instancia de canal predeterminada",

		// Skills
		MsgSkillsUpdateNotSupported: "skills.update no es compatible con skills basadas en archivos",
		MsgCannotResolveSkillID:     "no se puede resolver el ID de la skill basada en archivo",

		// Logs
		MsgInvalidLogAction: "la acción debe ser 'start' o 'stop'",

		// Config
		MsgRawConfigRequired: "la configuración raw es obligatoria",
		MsgRawPatchRequired:  "el patch raw es obligatorio",

		// Storage / File
		MsgCannotDeleteSkillsDir: "no se pueden eliminar los directorios de skills",
		MsgFailedToReadFile:      "error al leer el archivo",
		MsgFileNotFound:          "archivo no encontrado",
		MsgInvalidVersion:        "versión no válida",
		MsgVersionNotFound:       "versión no encontrada",
		MsgFailedToDeleteFile:    "error al eliminar",

		// OAuth
		MsgNoPendingOAuth:    "no hay flujo OAuth pendiente",
		MsgFailedToSaveToken: "error al guardar el token",

		// Status
		MsgStatusWorking:       "🔄 Estoy trabajando en tu solicitud... Por favor espera.",
		MsgStatusDetailed:      "🔄 Estoy trabajando en tu solicitud...\n%s (iteración %d)\nEjecutando desde hace: %s\n\nPor favor espera — responderé cuando termine.",
		MsgStatusPhaseThinking: "Fase: Pensando...",
		MsgStatusPhaseToolExec: "Fase: Ejecutando %s",
		MsgStatusPhaseTools:    "Fase: Ejecutando herramientas...",
		MsgStatusPhaseCompact:  "Fase: Compactando contexto...",
		MsgStatusPhaseDefault:  "Fase: Procesando...",
		MsgCancelledReply:      "✋ Cancelado. ¿Qué te gustaría hacer ahora?",
		MsgInjectedAck:         "Entendido, lo incorporaré a lo que estoy trabajando.",

		// Knowledge Graph
		MsgEntityIDRequired:       "entity_id es obligatorio",
		MsgEntityFieldsRequired:   "external_id, name y entity_type son obligatorios",
		MsgTextRequired:           "text es obligatorio",
		MsgProviderModelRequired:  "provider y model son obligatorios",
		MsgInvalidProviderOrModel: "provider o model no válido",

		// Builtin tool descriptions
		MsgToolReadFile:        "Leer el contenido de un archivo del workspace del agente por su ruta",
		MsgToolWriteFile:       "Escribir contenido en un archivo del workspace, creando directorios según sea necesario",
		MsgToolListFiles:       "Listar archivos y directorios en una ruta dentro del workspace",
		MsgToolEdit:            "Aplicar ediciones dirigidas de buscar y reemplazar en archivos existentes sin reescribir el archivo completo",
		MsgToolExec:            "Ejecutar un comando shell en el workspace y devolver stdout/stderr",
		MsgToolWebSearch:       "Buscar información en la web usando un motor de búsqueda (Brave o DuckDuckGo)",
		MsgToolWebFetch:        "Obtener una página web o endpoint de API y extraer su contenido en texto",
		MsgToolMemorySearch:    "Buscar en la memoria a largo plazo del agente usando similitud semántica",
		MsgToolMemoryGet:       "Recuperar un documento de memoria específico por su ruta de archivo",
		MsgToolKGSearch:        "Buscar entidades, relaciones y observaciones en el grafo de conocimiento del agente",
		MsgToolReadImage:       "Analizar imágenes usando un proveedor LLM con capacidad de visión",
		MsgToolReadDocument:    "Analizar documentos (PDF, Word, Excel, PowerPoint, CSV, etc.) usando un proveedor LLM con capacidad de documentos",
		MsgToolCreateImage:     "Generar imágenes a partir de prompts de texto usando un proveedor de generación de imágenes",
		MsgToolReadAudio:       "Analizar archivos de audio (voz, música, sonidos) usando un proveedor LLM con capacidad de audio",
		MsgToolReadVideo:       "Analizar archivos de video usando un proveedor LLM con capacidad de video",
		MsgToolCreateVideo:     "Generar videos a partir de descripciones de texto usando IA",
		MsgToolCreateAudio:     "Generar música o efectos de sonido a partir de descripciones de texto usando IA",
		MsgToolTTS:             "Convertir texto en audio de voz con sonido natural",
		MsgToolBrowser:         "Automatizar interacciones del navegador: navegar páginas, hacer clic en elementos, rellenar formularios, capturar pantallas",
		MsgToolSessionsList:    "Listar sesiones de chat activas en todos los canales",
		MsgToolSessionStatus:   "Obtener el estado actual y metadatos de una sesión de chat específica",
		MsgToolSessionsHistory: "Recuperar el historial de mensajes de una sesión de chat específica",
		MsgToolSessionsSend:    "Enviar un mensaje a una sesión de chat activa en nombre del agente",
		MsgToolMessage:         "Enviar un mensaje proactivo a un usuario en un canal conectado (Telegram, Discord, etc.)",
		MsgToolCron:            "Programar o gestionar tareas recurrentes usando expresiones cron, horarios o intervalos",
		MsgToolSpawn:           "Crear un subagente para trabajo en segundo plano o delegar una tarea a un agente vinculado",
		MsgToolSkillSearch:     "Buscar skills disponibles por palabra clave o descripción para encontrar capacidades relevantes",
		MsgToolUseSkill:        "Activar una skill para usar sus capacidades especializadas (marcador de rastreo)",
		MsgToolSkillManage:     "Crear, editar o eliminar skills desde la experiencia de conversación",
		MsgToolPublishSkill:    "Registrar un directorio de skill en la base de datos del sistema, haciéndolo descubrible",
		MsgToolTeamTasks:       "Ver, crear, actualizar y completar tareas en el tablero de tareas del equipo",

		MsgSkillNudgePostscript: "Esta tarea involucró varios pasos. ¿Quieres que guarde el proceso como una skill reutilizable? Responde **\"guardar como skill\"** o **\"omitir\"**.",
		MsgSkillNudge70Pct:      "[Sistema] Estás al 70% de tu presupuesto de iteraciones. Considera si algún patrón de esta sesión sería una buena skill.",
		MsgSkillNudge90Pct:      "[Sistema] Estás al 90% de tu presupuesto de iteraciones. Si esta sesión involucró patrones reutilizables, considera guardarlos como skill antes de terminar.",
	})
}
