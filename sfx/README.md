# 🔊 Pasta de Efeitos Sonoros Customizados (SFX)

Coloque seus arquivos de áudio de efeitos sonoros nesta pasta para que o **EdgeGo Voice** os utilize automaticamente nas suas tags de texto.

---

## 📁 Nomenclatura dos Arquivos

O sistema busca os arquivos na seguinte ordem de prioridade:

1. **Específico por Voz**: `{NomeDaVoz}_{efeito}.mp3` ou `{NomeDaVoz}_{efeito}.wav`
   - Exemplo: `pt-BR-FranciscaNeural_teclado.wav`
   - Exemplo: `Francisca_notificacao.mp3`

2. **Genérico (Qualquer Voz)**: `{efeito}.mp3` ou `{efeito}.wav`
   - Exemplo: `efeito-teclado.wav`
   - Exemplo: `callcenter-ambiente.wav`
   - Exemplo: `ruido-fundo.wav`
   - Exemplo: `aplausos.mp3`
   - Exemplo: `notificacao.mp3`

---

## 🏷️ Como Usar no Texto / API:

Qualquer arquivo que você colocar aqui fica disponível imediatamente via tag de texto:

- Se você tiver `sfx/efeito-teclado.wav` $\rightarrow$ use no texto: `Só um momento [teclado:3s] localizei seu cadastro!`
- Se você tiver `sfx/notificacao.wav` $\rightarrow$ use no texto: `[som:notificacao] Você recebeu uma mensagem.`
- Se você tiver `sfx/aplausos.mp3` $\rightarrow$ use no texto: `Parabéns pelo resultado! [som:aplausos]`
- Se você tiver `sfx/meu_efeito.mp3` $\rightarrow$ use no texto: `[som:meu_efeito] texto aqui.`

---

## ⚡ Formatos Suportados:
- `.mp3` (Recomendado: 24kHz ou 44.1kHz mono/stereo)
- `.wav`
- `.opus` / `.ogg`
- `.aac`
