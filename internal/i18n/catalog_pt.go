package i18n

func init() {
	register(LocalePT, map[string]string{
		// Common validation
		MsgRequired:         "%s é obrigatório",
		MsgInvalidID:        "ID de %s inválido",
		MsgNotFound:         "%s não encontrado: %s",
		MsgAlreadyExists:    "%s já existe: %s",
		MsgInvalidRequest:   "requisição inválida: %s",
		MsgInvalidJSON:      "JSON inválido",
		MsgUnauthorized:     "não autorizado",
		MsgPermissionDenied: "permissão negada: papel insuficiente para %s",
		MsgInternalError:    "erro interno: %s",
		MsgInvalidSlug:      "%s deve ser um slug válido (apenas letras minúsculas, números e hífens)",
		MsgFailedToList:     "falha ao listar %s",
		MsgFailedToCreate:   "falha ao criar %s: %s",
		MsgFailedToUpdate:   "falha ao atualizar %s: %s",
		MsgFailedToDelete:   "falha ao excluir %s: %s",
		MsgFailedToSave:     "falha ao salvar %s: %s",
		MsgInvalidUpdates:   "atualizações inválidas",

		// Agent
		MsgAgentNotFound:       "agente não encontrado: %s",
		MsgCannotDeleteDefault: "não é possível excluir o agente padrão",
		MsgUserCtxRequired:     "contexto de usuário é obrigatório",

		// Chat
		MsgRateLimitExceeded: "limite de requisições excedido — aguarde",
		MsgNoUserMessage:     "nenhuma mensagem do usuário encontrada",
		MsgUserIDRequired:    "user_id é obrigatório",
		MsgMsgRequired:       "mensagem é obrigatória",

		// Channel instances
		MsgInvalidChannelType: "channel_type inválido",
		MsgInstanceNotFound:   "instância não encontrada",

		// Cron
		MsgJobNotFound:     "job não encontrado",
		MsgInvalidCronExpr: "expressão cron inválida: %s",

		// Config
		MsgConfigHashMismatch: "a configuração foi alterada (hash incompatível)",

		// Exec approval
		MsgExecApprovalDisabled: "aprovação de execução não está habilitada",

		// Pairing
		MsgSenderChannelRequired: "senderId e channel são obrigatórios",
		MsgCodeRequired:          "código é obrigatório",
		MsgSenderIDRequired:      "sender_id é obrigatório",

		// HTTP API
		MsgInvalidAuth:           "autenticação inválida",
		MsgMsgsRequired:          "messages é obrigatório",
		MsgUserIDHeader:          "cabeçalho X-ArgoClaw-User-Id é obrigatório",
		MsgFileTooLarge:          "arquivo muito grande ou formulário multipart inválido",
		MsgMissingFileField:      "campo 'file' ausente",
		MsgInvalidFilename:       "nome de arquivo inválido",
		MsgChannelKeyReq:         "channel e key são obrigatórios",
		MsgMethodNotAllowed:      "método não permitido",
		MsgStreamingNotSupported: "streaming não suportado",
		MsgOwnerOnly:             "apenas o proprietário pode %s",
		MsgNoAccess:              "sem acesso a este %s",
		MsgAlreadySummoning:      "o agente já está sendo invocado",
		MsgSummoningUnavailable:  "invocação não disponível",
		MsgNoDescription:         "o agente não tem descrição para reinvocação",
		MsgInvalidPath:           "caminho inválido",

		// Scheduler
		MsgQueueFull:    "fila da sessão está cheia",
		MsgShuttingDown: "o gateway está encerrando, tente novamente em breve",

		// Provider
		MsgProviderReqFailed: "%s: requisição falhou: %s",

		// Unknown method
		MsgUnknownMethod: "método desconhecido: %s",

		// Not implemented
		MsgNotImplemented: "%s ainda não implementado",

		// Agent links
		MsgLinksNotConfigured:   "links do agente não configurados",
		MsgInvalidDirection:     "a direção deve ser outbound, inbound ou bidirectional",
		MsgSourceTargetSame:     "origem e destino devem ser agentes diferentes",
		MsgCannotDelegateOpen:   "não é possível delegar para agentes abertos — apenas agentes predefinidos podem ser alvos de delegação",
		MsgNoUpdatesProvided:    "nenhuma atualização fornecida",
		MsgInvalidLinkStatus:    "o status deve ser active ou disabled",

		// Teams
		MsgTeamsNotConfigured:   "equipes não configuradas",
		MsgAgentIsTeamLead:      "o agente já é o líder da equipe",
		MsgCannotRemoveTeamLead: "não é possível remover o líder da equipe",

		// Delegations
		MsgDelegationsUnavailable: "delegações não disponíveis",

		// Channels
		MsgCannotDeleteDefaultInst: "não é possível excluir a instância de canal padrão",

		// Skills
		MsgSkillsUpdateNotSupported: "skills.update não é suportado para skills baseadas em arquivo",
		MsgCannotResolveSkillID:     "não é possível resolver o ID da skill baseada em arquivo",

		// Logs
		MsgInvalidLogAction: "a ação deve ser 'start' ou 'stop'",

		// Config
		MsgRawConfigRequired: "configuração raw é obrigatória",
		MsgRawPatchRequired:  "patch raw é obrigatório",

		// Storage / File
		MsgCannotDeleteSkillsDir: "não é possível excluir diretórios de skills",
		MsgFailedToReadFile:      "falha ao ler o arquivo",
		MsgFileNotFound:          "arquivo não encontrado",
		MsgInvalidVersion:        "versão inválida",
		MsgVersionNotFound:       "versão não encontrada",
		MsgFailedToDeleteFile:    "falha ao excluir",

		// OAuth
		MsgNoPendingOAuth:    "nenhum fluxo OAuth pendente",
		MsgFailedToSaveToken: "falha ao salvar o token",

		// Status
		MsgStatusWorking:       "🔄 Estou trabalhando na sua solicitação... Aguarde.",
		MsgStatusDetailed:      "🔄 Estou trabalhando na sua solicitação...\n%s (iteração %d)\nExecutando há: %s\n\nAguarde — responderei quando terminar.",
		MsgStatusPhaseThinking: "Fase: Pensando...",
		MsgStatusPhaseToolExec: "Fase: Executando %s",
		MsgStatusPhaseTools:    "Fase: Executando ferramentas...",
		MsgStatusPhaseCompact:  "Fase: Compactando contexto...",
		MsgStatusPhaseDefault:  "Fase: Processando...",
		MsgCancelledReply:      "✋ Cancelado. O que gostaria de fazer agora?",
		MsgInjectedAck:         "Entendido, vou incorporar isso ao que estou trabalhando.",

		// Knowledge Graph
		MsgEntityIDRequired:       "entity_id é obrigatório",
		MsgEntityFieldsRequired:   "external_id, name e entity_type são obrigatórios",
		MsgTextRequired:           "text é obrigatório",
		MsgProviderModelRequired:  "provider e model são obrigatórios",
		MsgInvalidProviderOrModel: "provider ou model inválido",

		// Builtin tool descriptions
		MsgToolReadFile:        "Ler o conteúdo de um arquivo do workspace do agente pelo caminho",
		MsgToolWriteFile:       "Escrever conteúdo em um arquivo no workspace, criando diretórios conforme necessário",
		MsgToolListFiles:       "Listar arquivos e diretórios em um caminho dentro do workspace",
		MsgToolEdit:            "Aplicar edições direcionadas de busca e substituição em arquivos existentes sem reescrever o arquivo inteiro",
		MsgToolExec:            "Executar um comando shell no workspace e retornar stdout/stderr",
		MsgToolWebSearch:       "Pesquisar informações na web usando um mecanismo de busca (Brave ou DuckDuckGo)",
		MsgToolWebFetch:        "Buscar uma página web ou endpoint de API e extrair seu conteúdo em texto",
		MsgToolMemorySearch:    "Pesquisar na memória de longo prazo do agente usando similaridade semântica",
		MsgToolMemoryGet:       "Recuperar um documento de memória específico pelo caminho do arquivo",
		MsgToolKGSearch:        "Pesquisar entidades, relacionamentos e observações no grafo de conhecimento do agente",
		MsgToolReadImage:       "Analisar imagens usando um provedor LLM com capacidade de visão",
		MsgToolReadDocument:    "Analisar documentos (PDF, Word, Excel, PowerPoint, CSV, etc.) usando um provedor LLM com capacidade de documentos",
		MsgToolCreateImage:     "Gerar imagens a partir de prompts de texto usando um provedor de geração de imagens",
		MsgToolReadAudio:       "Analisar arquivos de áudio (fala, música, sons) usando um provedor LLM com capacidade de áudio",
		MsgToolReadVideo:       "Analisar arquivos de vídeo usando um provedor LLM com capacidade de vídeo",
		MsgToolCreateVideo:     "Gerar vídeos a partir de descrições de texto usando IA",
		MsgToolCreateAudio:     "Gerar música ou efeitos sonoros a partir de descrições de texto usando IA",
		MsgToolTTS:             "Converter texto em áudio de fala com som natural",
		MsgToolBrowser:         "Automatizar interações no navegador: navegar páginas, clicar elementos, preencher formulários, capturar telas",
		MsgToolSessionsList:    "Listar sessões de chat ativas em todos os canais",
		MsgToolSessionStatus:   "Obter o status atual e metadados de uma sessão de chat específica",
		MsgToolSessionsHistory: "Recuperar o histórico de mensagens de uma sessão de chat específica",
		MsgToolSessionsSend:    "Enviar uma mensagem para uma sessão de chat ativa em nome do agente",
		MsgToolMessage:         "Enviar uma mensagem proativa para um usuário em um canal conectado (Telegram, Discord, etc.)",
		MsgToolCron:            "Agendar ou gerenciar tarefas recorrentes usando expressões cron, horários ou intervalos",
		MsgToolSpawn:           "Criar um subagente para trabalho em segundo plano ou delegar uma tarefa a um agente vinculado",
		MsgToolSkillSearch:     "Pesquisar skills disponíveis por palavra-chave ou descrição para encontrar capacidades relevantes",
		MsgToolUseSkill:        "Ativar uma skill para usar suas capacidades especializadas (marcador de rastreamento)",
		MsgToolSkillManage:     "Criar, editar ou excluir skills a partir da experiência de conversa",
		MsgToolPublishSkill:    "Registrar um diretório de skill no banco de dados do sistema, tornando-o descobrível",
		MsgToolTeamTasks:       "Visualizar, criar, atualizar e concluir tarefas no quadro de tarefas da equipe",

		MsgSkillNudgePostscript: "Esta tarefa envolveu várias etapas. Quer que eu salve o processo como uma skill reutilizável? Responda **\"salvar como skill\"** ou **\"pular\"**.",
		MsgSkillNudge70Pct:      "[Sistema] Você está em 70% do seu orçamento de iterações. Considere se algum padrão desta sessão daria uma boa skill.",
		MsgSkillNudge90Pct:      "[Sistema] Você está em 90% do seu orçamento de iterações. Se esta sessão envolveu padrões reutilizáveis, considere salvá-los como skill antes de concluir.",
	})
}
