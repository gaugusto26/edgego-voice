"use client";

import React, { useState, useRef, useEffect } from "react";
import { VoiceOption } from "@/types";
import { IonIcon } from "@/components/ui/ion-icon";
import { CountryFlag } from "@/components/ui/country-flag";
import { useLanguage } from "@/i18n/LanguageContext";

interface TagEditorToolbarProps {
  textareaRef: React.RefObject<HTMLTextAreaElement>;
  text: string;
  setText: React.Dispatch<React.SetStateAction<string>>;
  voices: VoiceOption[];
  isCapCutVoice?: boolean;
}

export function TagEditorToolbar({
  textareaRef,
  text,
  setText,
  voices,
  isCapCutVoice = false,
}: TagEditorToolbarProps) {
  const { t } = useLanguage();
  const [showPauseMenu, setShowPauseMenu] = useState(false);
  const [showVoiceMenu, setShowVoiceMenu] = useState(false);
  const [showSFXMenu, setShowSFXMenu] = useState(false);
  const [showEmotionMenu, setShowEmotionMenu] = useState(false);
  const [showSpeedMenu, setShowSpeedMenu] = useState(false);
  const [showPitchMenu, setShowPitchMenu] = useState(false);
  const [showPresetsMenu, setShowPresetsMenu] = useState(false);

  const pauseMenuRef = useRef<HTMLDivElement>(null);
  const voiceMenuRef = useRef<HTMLDivElement>(null);
  const sfxMenuRef = useRef<HTMLDivElement>(null);
  const emotionMenuRef = useRef<HTMLDivElement>(null);
  const speedMenuRef = useRef<HTMLDivElement>(null);
  const pitchMenuRef = useRef<HTMLDivElement>(null);
  const presetsMenuRef = useRef<HTMLDivElement>(null);

  // Fechar menus ao clicar fora
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (pauseMenuRef.current && !pauseMenuRef.current.contains(e.target as Node)) {
        setShowPauseMenu(false);
      }
      if (voiceMenuRef.current && !voiceMenuRef.current.contains(e.target as Node)) {
        setShowVoiceMenu(false);
      }
      if (sfxMenuRef.current && !sfxMenuRef.current.contains(e.target as Node)) {
        setShowSFXMenu(false);
      }
      if (emotionMenuRef.current && !emotionMenuRef.current.contains(e.target as Node)) {
        setShowEmotionMenu(false);
      }
      if (speedMenuRef.current && !speedMenuRef.current.contains(e.target as Node)) {
        setShowSpeedMenu(false);
      }
      if (pitchMenuRef.current && !pitchMenuRef.current.contains(e.target as Node)) {
        setShowPitchMenu(false);
      }
      if (presetsMenuRef.current && !presetsMenuRef.current.contains(e.target as Node)) {
        setShowPresetsMenu(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Inserir tag na posição exata do cursor da textarea ou envolver seleção
  const insertTagAtCursor = (openTag: string, closeTag = "") => {
    const textarea = textareaRef.current;
    if (!textarea) {
      setText((prev) => prev + " " + openTag + " " + closeTag);
      return;
    }

    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const currentText = textarea.value;

    const selected = currentText.substring(start, end);
    const before = currentText.substring(0, start);
    const after = currentText.substring(end);

    let insertion = "";
    if (closeTag !== "") {
      if (selected.length > 0) {
        insertion = `${openTag}${selected}${closeTag}`;
      } else {
        insertion = `${openTag}texto${closeTag}`;
      }
    } else {
      const needsSpaceBefore = before.length > 0 && !before.endsWith(" ") && !before.endsWith("\n");
      const needsSpaceAfter = after.length > 0 && !after.startsWith(" ") && !after.startsWith("\n");
      insertion = `${needsSpaceBefore ? " " : ""}${openTag}${needsSpaceAfter ? " " : " "}`;
    }

    const newText = before + insertion + after;
    setText(newText);

    setTimeout(() => {
      textarea.focus();
      const newCursorPos = start + insertion.length;
      textarea.setSelectionRange(newCursorPos, newCursorPos);
    }, 10);
  };

  // Limpar todas as tags do texto
  const handleClearTags = () => {
    const cleaned = text
      .replace(/\[(?:pausa|pause|break):\s*[^\]]+\]/gi, "")
      .replace(/\((?:pausa|pause|break):\s*[^)]+\)/gi, "")
      .replace(/<break\s+[^>]*\/?>/gi, "")
      .replace(/\[(?:voz|voice):\s*[^\]]+\]/gi, "")
      .replace(/<voice\s+[^>]*>/gi, "")
      .replace(/\[(?:velocidade|speed):\s*[^\]]+\]/gi, "")
      .replace(/\[(?:tom|pitch):\s*[^\]]+\]/gi, "")
      .replace(/\[(?:som|sfx|sound):\s*[^\]]+\]/gi, "")
      .replace(/\[\/?(?:telefone|callcenter|ambiente|escritorio|escritório|ruido|ruído|teclado)[^\]]*\]/gi, "")
      .replace(/\[\/?(?:emocao|emoção|emotion|estilo|style)[^\]]*\]/gi, "")
      .replace(/\[\/?(?:alegre|cheerful|triste|sad|bravo|irritado|angry|animado|empolgado|excited|calmo|calm|gritando|shouting|amigavel|amigável|friendly|esperancoso|esperançoso|hopeful|feliz|happy|serio|sério|firme|surpreso|surprise|tenso|medo|fear)\]/gi, "")
      .replace(/\[(?:hum|hmm|entendi|certo|deixa-ver|filler:[^\]]+)\]/gi, "")
      .replace(/<\/?(?:mstts:express-as|emphasis|say-as|sub|prosody)[^>]*>/gi, "")
      .replace(/\s+/g, " ")
      .trim();
    setText(cleaned);
  };

  // Carregar preset de Atendimento Telefônico com filtro DSP, callcenter e teclado
  const handleLoadTelephonyPreset = () => {
    const sample = `[telefone][callcenter]Olá! Seja bem-vindo ao suporte telefônico. [pausa: 500ms] Deixe-me consultar seu protocolo no sistema [teclado:3s]. Pronto, já localizei seu cadastro! Como posso ajudar hoje?[/callcenter][/telefone]`;
    setText(sample);
  };

  // Carregar preset de Atendimento Callcenter com digitação procedural e pausas reais
  const handleLoadCallcenterPreset = () => {
    const sample = `[callcenter]Olá! Seja bem-vindo ao atendimento EdgeGo Voice. [pausa: 1s] Só um momento enquanto consulto seu cadastro no sistema [teclado:4s]. Pronto, já encontrei seus dados! Como posso te ajudar hoje?[/callcenter]`;
    setText(sample);
  };

  // Carregar preset de Diálogo entre 2 vozes com pausas
  const handleLoadDialogPreset = () => {
    const voice1 = voices[0]?.id || "pt-BR-FranciscaNeural";
    const voice2 = voices[1]?.id || "pt-BR-AntonioNeural";
    const sample = `Olá! Tudo bem com você? [pausa: 1.5s] [voz: ${voice2}] Olá! Tudo ótimo por aqui, pronto para demonstrar o motor nativo em Go. [pausa: 1s] [voz: ${voice1}] Maravilha, síntese perfeita com zero latência!`;
    setText(sample);
  };

  return (
    <div className="flex flex-wrap items-center justify-between gap-1.5 p-1.5 rounded-t-md bg-secondary/40 border-b border-border/70 text-xs">
      <div className="flex flex-wrap items-center gap-1">
        {/* 1. Botão Efeitos SFX & Ambiente */}
        <div className="relative" ref={sfxMenuRef}>
          <button
            type="button"
            onClick={() => setShowSFXMenu(!showSFXMenu)}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded bg-secondary/80 hover:bg-secondary text-foreground border border-border/60 transition-colors"
            title={t("toolbarSFX")}
          >
            <IonIcon name="volume-medium-outline" className="text-xs text-amber-400" />
            <span className="font-medium text-[11px]">{t("toolbarSFX")}</span>
            <IonIcon name="chevron-down-outline" className="text-[10px] text-muted-foreground" />
          </button>

          {showSFXMenu && (
            <div className="absolute left-0 top-full mt-1 w-64 bg-zinc-900 border border-border rounded-md shadow-2xl z-50 p-1 space-y-0.5 animate-in fade-in zoom-in-95 duration-100 max-h-96 overflow-y-auto">
              <div className="px-2 py-1 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                <IonIcon name="layers-outline" className="text-xs" />
                <span>{t("sfxMenuTitle")}</span>
              </div>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[telefone]", "[/telefone]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="call-outline" className="text-xs text-muted-foreground group-hover:text-blue-400 transition-colors" />
                  <span>Linha Telefônica (DSP)</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[telefone]</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[callcenter]", "[/callcenter]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="headset-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>Callcenter Ambiente</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[callcenter]</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[teclado]", "[/teclado]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="keypad-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>{t("sfxKeyboard")}</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[teclado]</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[ruido]", "[/ruido]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="mic-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>{t("sfxBackgroundNoise")}</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[ruido]</span>
              </button>

              <div className="my-1 border-t border-border/60" />
              <div className="px-2 py-1 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                <IonIcon name="timer-outline" className="text-xs" />
                <span>{t("sfxIntervalsSection")}</span>
              </div>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[teclado:5s]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="keypad-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>{t("sfxKeyboardTimed")}</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[teclado:5s]</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[callcenter:3s]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="headset-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>{t("sfxCallcenterTimed")}</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[callcenter:3s]</span>
              </button>

              <div className="my-1 border-t border-border/60" />
              <div className="px-2 py-1 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
                <IonIcon name="chatbubbles-outline" className="text-xs" />
                <span>{t("sfxFillersSection")}</span>
              </div>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[hum]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="help-circle-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>{t("sfxHesitation")}</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[hum]</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[entendi]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="checkmark-done-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>{t("sfxConfirmation")}</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[entendi]</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[deixa-ver]");
                  setShowSFXMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-zinc-800 flex items-center justify-between text-xs text-foreground group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="search-outline" className="text-xs text-muted-foreground group-hover:text-amber-400 transition-colors" />
                  <span>{t("sfxConsultation")}</span>
                </span>
                <span className="text-[10px] text-muted-foreground font-mono">[deixa-ver]</span>
              </button>
            </div>
          )}
        </div>

        {/* 1.5 Botão Sentimento / Expressividade CapCut - Ativo apenas para modelos CapCut */}
        {isCapCutVoice && (
          <div className="relative" ref={emotionMenuRef}>
            <button
              type="button"
              onClick={() => setShowEmotionMenu(!showEmotionMenu)}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded bg-purple-500/10 hover:bg-purple-500/20 text-purple-200 border border-purple-500/30 transition-colors"
              title={t("toolbarEmotion")}
            >
              <IonIcon name="sparkles-outline" className="text-xs text-purple-400" />
              <span className="font-medium text-[11px]">{t("toolbarEmotion")}</span>
              <IonIcon name="chevron-down-outline" className="text-[10px] text-purple-300/70" />
            </button>

            {showEmotionMenu && (
              <div className="absolute left-0 top-full mt-1 w-56 bg-zinc-900 border border-purple-500/30 rounded-md shadow-2xl z-50 p-1 space-y-0.5 animate-in fade-in zoom-in-95 duration-100">
                <div className="px-2 py-1 text-[10px] font-semibold text-purple-300 uppercase tracking-wider flex items-center gap-1.5">
                  <IonIcon name="heart-outline" className="text-xs" />
                  <span>{t("emotionExpressivenessSection")}</span>
                </div>
                {[
                  { label: t("emotionHappyPill"), tag: "[emocao:feliz]", close: "[/emocao]", icon: "😊" },
                  { label: t("emotionCalmPill"), tag: "[emocao:calmo]", close: "[/emocao]", icon: "😌" },
                  { label: t("emotionFirmPill"), tag: "[emocao:firme]", close: "[/emocao]", icon: "😠" },
                  { label: t("emotionSadPill"), tag: "[emocao:triste]", close: "[/emocao]", icon: "😢" },
                  { label: t("emotionSurprisePill"), tag: "[emocao:surpreso]", close: "[/emocao]", icon: "😲" },
                  { label: t("emotionFearPill"), tag: "[emocao:tenso]", close: "[/emocao]", icon: "😱" },
                ].map((em) => (
                  <button
                    key={em.tag}
                    type="button"
                    onClick={() => {
                      insertTagAtCursor(em.tag, em.close);
                      setShowEmotionMenu(false);
                    }}
                    className="w-full text-left px-2 py-1.5 rounded hover:bg-purple-950/50 flex items-center justify-between text-xs text-foreground group"
                  >
                    <span className="flex items-center gap-2">
                      <span>{em.icon}</span>
                      <span>{em.label}</span>
                    </span>
                    <span className="text-[10px] text-purple-400/80 font-mono">{em.tag}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {/* 2. Botão Pausa */}
        <div className="relative" ref={pauseMenuRef}>
          <button
            type="button"
            onClick={() => setShowPauseMenu(!showPauseMenu)}
            className="flex items-center gap-1 px-2 py-1 rounded bg-secondary/80 hover:bg-secondary text-foreground border border-border/60 transition-colors"
          >
            <IonIcon name="pause-outline" className="text-xs text-muted-foreground" />
            <span>{t("insertPause")}</span>
            <IonIcon name="chevron-down-outline" className="text-[10px] text-muted-foreground" />
          </button>

          {showPauseMenu && (
            <div className="absolute left-0 top-full mt-1 w-48 bg-zinc-900 border border-border rounded-md shadow-xl z-50 p-1 space-y-0.5 animate-in fade-in zoom-in-95 duration-100">
              <div className="px-2 py-0.5 text-[9px] font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1">
                <IonIcon name="time-outline" className="text-[10px]" />
                <span>Pausas em Silêncio</span>
              </div>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[pausa: 500ms]");
                  setShowPauseMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="time-outline" className="text-xs text-muted-foreground" />
                <span>500ms</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[pausa: 1s]");
                  setShowPauseMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="time-outline" className="text-xs text-muted-foreground" />
                <span>1.0s</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[pausa: 2s]");
                  setShowPauseMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="time-outline" className="text-xs text-muted-foreground" />
                <span>2.0s</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[pausa: 3s]");
                  setShowPauseMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="time-outline" className="text-xs text-muted-foreground" />
                <span>3.0s</span>
              </button>

              <div className="my-1 border-t border-border/60" />
              <div className="px-2 py-0.5 text-[9px] font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1">
                <IonIcon name="keypad-outline" className="text-[10px]" />
                <span>Pausa com Teclado</span>
              </div>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[pausa: 500ms] [teclado:2s]");
                  setShowPauseMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center justify-between group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="keypad-outline" className="text-xs text-amber-400" />
                  <span>Pausa + Teclado (2s)</span>
                </span>
                <span className="text-[9px] text-muted-foreground font-mono">[teclado:2s]</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[pausa: 1s] [teclado:4s]");
                  setShowPauseMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center justify-between group"
              >
                <span className="flex items-center gap-1.5">
                  <IonIcon name="keypad-outline" className="text-xs text-amber-400" />
                  <span>Pausa + Teclado (4s)</span>
                </span>
                <span className="text-[9px] text-muted-foreground font-mono">[teclado:4s]</span>
              </button>
            </div>
          )}
        </div>

        {/* 3. Botão Trocar Voz */}
        <div className="relative" ref={voiceMenuRef}>
          <button
            type="button"
            onClick={() => setShowVoiceMenu(!showVoiceMenu)}
            className="flex items-center gap-1 px-2 py-1 rounded bg-secondary/80 hover:bg-secondary text-foreground border border-border/60 transition-colors"
          >
            <IonIcon name="mic-outline" className="text-xs text-muted-foreground" />
            <span>{t("insertVoice")}</span>
            <IonIcon name="chevron-down-outline" className="text-[10px] text-muted-foreground" />
          </button>

          {showVoiceMenu && (
            <div className="absolute left-0 top-full mt-1 w-56 max-h-60 overflow-y-auto bg-zinc-900 border border-border rounded-md shadow-xl z-50 p-1 space-y-0.5 animate-in fade-in zoom-in-95 duration-100">
              {voices.map((v) => {
                const shortName = v.name.split(" ")[0];
                return (
                  <button
                    key={v.id}
                    type="button"
                    onClick={() => {
                      insertTagAtCursor(`[voz: ${v.id}]`);
                      setShowVoiceMenu(false);
                    }}
                    className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary flex items-center justify-between text-xs text-foreground"
                  >
                    <div className="flex items-center gap-1.5 truncate">
                      {v.country_code && (
                        <CountryFlag countryCode={v.country_code} className="w-3.5 h-2.5 rounded-[1px]" />
                      )}
                      <span className="truncate">{shortName}</span>
                    </div>
                    <span className="text-[10px] text-muted-foreground uppercase">{v.gender === "female" ? "♀" : "♂"}</span>
                  </button>
                );
              })}
            </div>
          )}
        </div>

        {/* 4. Botão Velocidade */}
        <div className="relative" ref={speedMenuRef}>
          <button
            type="button"
            onClick={() => setShowSpeedMenu(!showSpeedMenu)}
            className="flex items-center gap-1 px-2 py-1 rounded bg-secondary/80 hover:bg-secondary text-foreground border border-border/60 transition-colors"
          >
            <IonIcon name="speedometer-outline" className="text-xs text-muted-foreground" />
            <span>{t("insertSpeed")}</span>
            <IonIcon name="chevron-down-outline" className="text-[10px] text-muted-foreground" />
          </button>

          {showSpeedMenu && (
            <div className="absolute left-0 top-full mt-1 w-40 bg-zinc-900 border border-border rounded-md shadow-xl z-50 p-1 space-y-0.5 animate-in fade-in zoom-in-95 duration-100">
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[velocidade: 0.8x]");
                  setShowSpeedMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="play-back-outline" className="text-xs text-muted-foreground" />
                <span>0.8x (Lenta)</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[velocidade: 1.2x]");
                  setShowSpeedMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="play-forward-outline" className="text-xs text-muted-foreground" />
                <span>1.2x (Rápida)</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[velocidade: 1.5x]");
                  setShowSpeedMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="flash-outline" className="text-xs text-muted-foreground" />
                <span>1.5x (Muito Rápida)</span>
              </button>
            </div>
          )}
        </div>

        {/* 5. Botão Tom */}
        <div className="relative" ref={pitchMenuRef}>
          <button
            type="button"
            onClick={() => setShowPitchMenu(!showPitchMenu)}
            className="flex items-center gap-1 px-2 py-1 rounded bg-secondary/80 hover:bg-secondary text-foreground border border-border/60 transition-colors"
          >
            <IonIcon name="musical-notes-outline" className="text-xs text-muted-foreground" />
            <span>{t("insertPitch")}</span>
            <IonIcon name="chevron-down-outline" className="text-[10px] text-muted-foreground" />
          </button>

          {showPitchMenu && (
            <div className="absolute left-0 top-full mt-1 w-36 bg-zinc-900 border border-border rounded-md shadow-xl z-50 p-1 space-y-0.5 animate-in fade-in zoom-in-95 duration-100">
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[tom: -10Hz]");
                  setShowPitchMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="arrow-down-outline" className="text-xs text-muted-foreground" />
                <span>-10Hz (Grave)</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  insertTagAtCursor("[tom: +10Hz]");
                  setShowPitchMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="arrow-up-outline" className="text-xs text-muted-foreground" />
                <span>+10Hz (Agudo)</span>
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Presets & Limpeza */}
      <div className="flex items-center gap-1">
        <div className="relative" ref={presetsMenuRef}>
          <button
            type="button"
            onClick={() => setShowPresetsMenu(!showPresetsMenu)}
            className="flex items-center gap-1 px-2 py-1 rounded bg-secondary/60 hover:bg-secondary text-muted-foreground hover:text-foreground transition-colors"
            title={t("toolbarPresets")}
          >
            <IonIcon name="bulb-outline" className="text-xs" />
            <span className="text-[11px]">{t("toolbarPresets")}</span>
            <IonIcon name="chevron-down-outline" className="text-[10px]" />
          </button>

          {showPresetsMenu && (
            <div className="absolute right-0 top-full mt-1 w-52 bg-zinc-900 border border-border rounded-md shadow-xl z-50 p-1 space-y-0.5 animate-in fade-in zoom-in-95 duration-100">
              <button
                type="button"
                onClick={() => {
                  handleLoadTelephonyPreset();
                  setShowPresetsMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="call-outline" className="text-xs text-blue-400" />
                <span>Telefonia PABX Real</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  handleLoadCallcenterPreset();
                  setShowPresetsMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="headset-outline" className="text-xs text-amber-400" />
                <span>Callcenter + Teclado</span>
              </button>
              <button
                type="button"
                onClick={() => {
                  handleLoadDialogPreset();
                  setShowPresetsMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 rounded hover:bg-secondary text-xs text-foreground flex items-center gap-1.5"
              >
                <IonIcon name="chatbubbles-outline" className="text-xs text-muted-foreground" />
                <span>{t("dialogPreset")}</span>
              </button>
            </div>
          )}
        </div>

        <button
          type="button"
          onClick={handleClearTags}
          className="flex items-center gap-1 px-2 py-1 rounded bg-secondary/40 hover:bg-secondary text-muted-foreground hover:text-rose-400 transition-colors"
          title={t("clearTags")}
        >
          <IonIcon name="trash-outline" className="text-xs" />
          <span className="text-[11px]">{t("clearTags")}</span>
        </button>
      </div>
    </div>
  );
}
