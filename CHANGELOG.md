# 📜 Histórico de Versões (Changelog)

Todas as mudanças notáveis no projeto **EdgeGo Voice** são documentadas neste arquivo.

O formato é baseado no [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/) e adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [1.2.0] - 2026-09-16

### ✨ Adicionado
- **Geração e Extração Completa de Legendas (`internal/subtitles`):**
  - Suporte a múltiplos formatos de legenda de mercado: **SubRip (`.srt`)**, **WebVTT (`.vtt`)** e **JSON Estruturado** com timestamps em milissegundos por palavra e sentença.
  - Formatação com alocação zero de memória usando `strings.Builder` e pré-cálculo de capacidade (`golang-performance`).
  - Algoritmo de fallback inteligente com divisão por fronteiras de sentença e interpolação ponderada de caracteres/palavras.
  - Orquestração com acumulação cronológica de offsets (`accumulatedOffsetMs`) para sincronizar legendas em áudios compostos com pausas (`[pausa: ...]`), efeitos SFX (`[teclado: ...]`) e blocos de ambiente (`[callcenter]`).
- **Novas Rotas de Legendas na API REST:**
  - `POST /v1/audio/subtitles`: Gera diretamente legendas no formato solicitado (`srt`, `vtt` ou `json`).
  - `POST /v1/persona/{id}/subtitles`: Geração de legendas personalizada para Personas salvas.
  - Suporte ao parâmetro `with_subtitles: true` ou `response_format: "verbose_json"` na rota padrão `POST /v1/audio/speech`, retornando o áudio em base64 junto com metadados e estrutura completa de legendas.
  - Cabeçalho HTTP `X-Subtitles-Duration-Ms` exposto em todas as respostas de síntese.
- **Player com Karaokê em Tempo Real & Central de Exportação no Frontend:**
  - **Live Karaoke Banner:** Frase ativa destacada em tempo real com indicador de tempo e sincronização com `audioRef.currentTime`.
  - **Navegação Interativa por Sentenças:** Lista de frases clicáveis que permite ao usuário pular diretamente para qualquer ponto do áudio.
  - **Exportação Multiformato:** Botões para download do áudio original, arquivo `.srt`, arquivo `.vtt` e cópia da estrutura JSON com 1 clique.

### 🔒 Segurança & Higienização de Erros
- **Prevenção de Vazamento de Informações de Infraestrutura (CWE-209):**
  - Removidas referências a nomes de firewalls corporativos, regras de bloqueio de rede, URLs de CDN com tokens de autenticação assinados e nomes de arquivos internos (`.env`) de todas as mensagens de erro retornadas pela API e pelos logs do servidor.
  - O servidor agora responde com mensagens genéricas e seguras ao cliente (`502 Bad Gateway` com descrição padronizada), mantendo detalhes técnicos apenas em logs internos protegidos.
  - No Frontend (`PlaygroundView`), adicionada a função `sanitizeErrorMessage` que intercepta e mascara qualquer detalhe técnico residual antes de exibir alertas na interface, além de eliminar saídas inseguras no console do navegador (`console.error`).

---

## [1.1.0] - 2026-09-16

### ✨ Adicionado
- **Integração Nativa com o Motor CapCut / ByteDance Sami TTS (`internal/capcuttts`):**
  - Síntese de voz com modelos neurais ultra-realistas com suporte a emoções em português (`wenrou`, `empresario`, `luiz`, `youya`) e inglês (`emotional`).
  - Suporte a expressividade dinâmica inline via tags `[emocao:feliz]texto[/emocao]`, `[feliz]`, `[calmo]`, `[firme]`, `[triste]`, etc., mapeadas diretamente para as APIs dos motores CapCut e Edge TTS.
  - Seletor de Sentimentos no Voice Studio ativado dinamicamente e com exclusividade para modelos compatíveis com emoções (CapCut).
  - Suporte a Proxy HTTP/HTTPS via variável de ambiente `PROXY=` no `.env` para contornar restrições de rede no motor CapCut.
- **Streaming Bidirecional WebSocket para LLMs (`GET /v1/audio/stream`):**
  - **Ultra-Baixa Latência Conversacional (TTFB < 250ms):** Permite que agentes de IA (n8n, Dify, Typebot, Python) enviem fluxos de tokens de LLM e recebam chunks de áudio sentença a sentença.
  - **Tag-Aware Sentence Chunker (`internal/audioorchestrator`):** Máquina de estados que divide fluxos de tokens em sentenças sem jamais fragmentar tags (`[teclado:2s]`, `[pausa: 500ms]`) ou interromper envelopes de ambiente (`[callcenter]`) e linha telefônica (`[telefone]`).
- **Filtro Acústico DSP de Linha Telefônica & PABX (`[telefone]...[/telefone]`):**
  - Processamento passa-faixa (300Hz–3.400Hz), equalização de presença telefônica e compressão dinâmica para simular com perfeição canais telefônicos PSTN/GSM.
- **Simulação Procedural de Digitação Humana via Teclado (`internal/sfx`):**
  - Alternância orgânica entre gravações de teclas com rajadas (0.35s a 0.70s) e pausas de reflexão (250ms a 500ms) sob a tag `[teclado:Xs]`.
- **Cache Sharded Segmented-LRU (SLRU) & Deduplicação Concorrente (`internal/audiocache`):**
  - 16 shards com hash FNV-1a para eliminar contenção entre núcleos de CPU, proteção contra *scan pollution* e `singleflight` para evitar requisições idênticas simultâneas.
- **Voice Studio Workstation (Redesign Completo do Playground):**
  - Layout unificado de estúdio profissional, componente interativo AI Elements Persona em WebGL2 (@rive-app), waveform acoplado e visual minimalista.

### 🔄 Modificado
- **Sanitização Inteligente de Tags de Expressividade:**
  - O parser agora extrai as emoções das tags entre colchetes (`[feliz]`, `[emocao:calmo]`, etc.) e higieniza o texto falado, garantindo que o sintetizador aplique o estilo vocal sem jamais ler os colchetes em voz alta.
- **Persistência Atômica de Personas (`personas.json`):**
  - Implementada gravação em arquivo temporário com `os.Rename` atômico e permissões restritas `0600`, prevenindo corrupção de dados em caso de queda de energia.

### 🐛 Corrigido
- **Correção de Resposta HTTP 200 Prematura na Síntese:**
  - Corrigido fluxo em `handler_speech.go` onde o cabeçalho `200 OK` era enviado antes do término da síntese, gerando áudios vazios de 0 bytes e mascarando falhas no player. O servidor agora só confirma o status `200` após o áudio estar pronto, retornando `HTTP 502 Bad Gateway` com JSON descritivo em caso de erro.
- **Diagnóstico Transparente de Bloqueios de Firewall de Rede:**
  - Detecção proativa de interceptações de firewall corporativo (ex: FortiGate Application Control) no download do CDN ByteDance (`*.tiktokcdn.com`), alertando o usuário com instruções claras de configuração de Proxy.

### 🗑️ Removido
- **Eliminação Completa do Módulo de Micro-Expressões Legadas:**
  - Removidos áudios e dependências de micro-expressões vocais legadas (`[pigarro]`, `[risos]`, fillers), simplificando a arquitetura em favor de pausas naturais com silêncio puro (`[pausa: 500ms]`) e digitação procedural humana via teclado.
- **Integração Legada com Microsoft Azure Speech API:**
  - Removido pacote `internal/azuretts/`, chaves de API pagas e medidores de custo em dólar, operando 100% em modo nativo Go gratuito ($0.00).
- **Página e Menu de Métricas & Consumo:**
  - Removida a aba de navegação e componentes não utilizados, deixando o painel focado em Personas, Voice Studio e Documentação.

### 🔒 Segurança
- **Proteção Contra Timing Attacks (Ataques de Tempo):**
  - Validação de chaves de API globais e por Persona refatorada para comparação em tempo constante usando `crypto/subtle.ConstantTimeCompare`.
- **Mitigação de Negação de Serviço (Anti-DoS):**
  - Limitação estrita de leitura de payload HTTP com `http.MaxBytesReader` em endpoints de síntese (2MB) e gerenciamento de personas (1MB).
  - Limite de leitura defensivo configurado para conexões WebSocket com `conn.SetReadLimit(1 << 20)`.
- **Cabeçalhos de Segurança Defensivos (OWASP):**
  - Ativação do middleware `SecurityHeadersMiddleware` injetando `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`, `X-XSS-Protection: 1; mode=block` e `Referrer-Policy: strict-origin-when-cross-origin`.

---

## [1.0.0] - 2026-09-01

### 🚀 Lançamento Oficial: EdgeGo Voice (Go Native + Next.js)

Reescrita completa da arquitetura do projeto para uma plataforma nativa em **Go (Golang 1.22)** com painel administrativo integrado em **Next.js 14**, substituindo o protótipo legado em Python por um motor de altíssima performance e baixo consumo de recursos.

### ✨ Funcionalidades (Features)
- **Motor TTS em Golang 1.22:**
  - Streaming em tempo real de áudio binário via `http.Flusher` conectado diretamente aos WebSockets do Microsoft Edge TTS.
  - Consumo de memória ultrabaixo (~20MB de RAM) e alto throughput para concorrência massiva sem bloqueios de GIL.
- **100% Compatível com a API OpenAI (`/v1/audio/speech`):**
  - *Drop-in replacement* para SDKs oficiais da OpenAI (Python, Node.js, PHP, cURL) e plataformas de automação (n8n, Typebot, Dify, Flowise, Evolution API).
- **Sistema de Personas de Áudio:**
  - Gerenciamento de perfis pré-configurados de áudio (voz, formato, velocidade e pitch).
  - Rota dedicada de síntese direta: `POST /v1/persona/{id}/speech`.
  - Persistência atômica em arquivo JSON com suporte a concorrência (`sync.RWMutex`).
- **Cache LRU em Memória Thread-Safe:**
  - Respostas imediatas em **< 1ms** para requisições repetidas.
  - Chaveamento criptográfico SHA-256 (texto + voz + velocidade + formato + filtros).
  - Limite configurável de memória máxima (MB) e tempo de expiração TTL.
- **Higienizador Inteligente de Texto (Text Cleaner):**
  - Remoção automática de marcações Markdown (`**negrito**`, `# títulos`, links), URLs e caracteres de controle para fala limpa e natural.
- **Painel Administrativo SPA (Next.js 14):**
  - Dashboard dark mode com estética refinada e componentes **shadcn/ui**.
  - **Playground Interativo:** Síntese em tempo real com estatísticas de latência (ms), tamanho do arquivo, taxa de transferência e player de áudio com visualização de ondas (Waveform).
  - **Gerador de Snippets de Integração:** Exemplos de código prontos para cURL, Python (OpenAI SDK), JavaScript/TypeScript, Go e automações via Webhook (n8n/Evolution API).
  - **Catálogo de Vozes Neurais Multilíngues:** Suporte a vozes de alta fidelidade em português (Brasil e Portugal), inglês, espanhol, francês, alemão, italiano e japonês com identificação visual por bandeiras de países.
- **Distribuição em Container Único:**
  - Imagem Docker Alpine minimalista (~18MB) compilando o binário Go e servindo o bundle estático do Next.js na mesma porta (`5050`).

---

## [0.2.0] - 2025-10-02 (Legado - Protótipo Python)

- Suporte a vozes neurais multilinguais e mapeamento via `voices.json`.
- Conversão de formato de áudio via `pydub` e `ffmpeg`.
