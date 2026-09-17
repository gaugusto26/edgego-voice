"use client";

import React, { useState, useRef, useEffect, useCallback } from "react";
import { Persona, VoiceOption, SubtitleResult } from "@/types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { IonIcon } from "@/components/ui/ion-icon";
import { VoiceSelect } from "@/components/ui/voice-select";
import { WaveformPlayer } from "@/components/WaveformPlayer";
import { Textarea } from "@/components/ui/textarea";
import { TagEditorToolbar } from "@/components/TagEditorToolbar";
import { Persona as AiPersona, PersonaState } from "@/components/ai-elements/persona";
import { CountryFlag } from "@/components/ui/country-flag";
import { formatBytes } from "@/lib/utils";
import { useLanguage } from "@/i18n/LanguageContext";

interface PlaygroundViewProps {
  personas: Persona[];
  voices: VoiceOption[];
  serverUrl: string;
  apiKey: string;
  selectedPersonaId?: string;
}

interface ServerTelemetry {
  goroutines: number;
  alloc_mb: number;
  sys_mb: number;
  heap_inuse_mb: number;
  cache_items: number;
  cache_bytes: number;
}

export function PlaygroundView({
  personas,
  voices,
  serverUrl,
  apiKey,
  selectedPersonaId: initialPersonaId,
}: PlaygroundViewProps) {
  const { t } = useLanguage();
  const [selectedPersonaId, setSelectedPersonaId] = useState<string>(
    initialPersonaId || ""
  );

  const [voice, setVoice] = useState(
    voices[0]?.id || "pt-BR-FranciscaNeural"
  );
  const [format, setFormat] = useState("mp3");
  const [speed, setSpeed] = useState(1.0);
  const [pitchHz, setPitchHz] = useState(0);
  const [breakCommaMs, setBreakCommaMs] = useState(150);
  const [breakPeriodMs, setBreakPeriodMs] = useState(350);
  const [isTelephony, setIsTelephony] = useState(false);
  const [emotion, setEmotion] = useState("");
  const [emotionScale, setEmotionScale] = useState(1.0);
  const [text, setText] = useState(
    "[callcenter]Olá! Seja bem-vindo ao atendimento EdgeGo Voice. [pausa: 1s] Só um momento enquanto consulto seu cadastro no sistema [teclado:4s]. Pronto, já encontrei seus dados! Como posso te ajudar hoje?[/callcenter]"
  );

  const parseHz = (val?: string) => {
    if (!val) return 0;
    const num = parseInt(val.replace("Hz", "").replace("hz", ""), 10);
    return isNaN(num) ? 0 : num;
  };

  const parseMs = (val?: string, fallback = 0) => {
    if (!val) return fallback;
    const num = parseInt(val.replace("ms", "").replace("s", "000"), 10);
    return isNaN(num) ? fallback : num;
  };

  // Sincronizar com Persona selecionada
  const applyPersona = useCallback((personaId: string) => {
    setSelectedPersonaId(personaId);
    if (!personaId) return;

    const p = personas.find((x) => x.id === personaId);
    if (p) {
      setVoice(p.voice);
      setFormat(p.format || "mp3");
      setSpeed(p.speed || 1.0);
      setPitchHz(parseHz(p.pitch));
      setBreakCommaMs(parseMs(p.break_comma, 150));
      setBreakPeriodMs(parseMs(p.break_period, 350));
      setIsTelephony(p.telephony || false);
      setEmotion(p.emotion || "");
      setEmotionScale(p.emotion_scale ? parseFloat(p.emotion_scale) : 1.0);
    }
  }, [personas]);

  useEffect(() => {
    if (initialPersonaId) {
      applyPersona(initialPersonaId);
    }
  }, [initialPersonaId, applyPersona]);

  const [isLoading, setIsLoading] = useState(false);
  const [isPlayingAudio, setIsPlayingAudio] = useState(false);
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const [subtitles, setSubtitles] = useState<SubtitleResult | null>(null);
  const [telemetry, setTelemetry] = useState<ServerTelemetry | null>(null);

  const [alertInfo, setAlertInfo] = useState<{
    variant: "default" | "destructive";
    title: string;
    message: string;
  } | null>(null);

  const [metrics, setMetrics] = useState<{
    ttfb: number;
    totalTime: number;
    size: number;
    cacheHit?: boolean;
  } | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const alertTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const sanitizeErrorMessage = (msg?: string): string => {
    if (!msg) return t("errorGenericSynthesis");

    // Bloqueia qualquer termo técnico de infraestrutura, URLs, firewalls, CDN ou .env
    const sensitivePatterns = [
      /https?:\/\/[^\s]+/i,
      /firewall/i,
      /fortigate/i,
      /tiktok/i,
      /bytedance/i,
      /segmento\s+\d+/i,
      /\.env/i,
      /sami/i,
      /proxy/i,
      /cdn/i,
      /status\s+http\s+\d+/i,
      /connection\s+refused/i,
      /dial\s+tcp/i,
      /403/i,
      /502/i,
    ];

    for (const pattern of sensitivePatterns) {
      if (pattern.test(msg)) {
        return t("errorServiceUnavailable");
      }
    }

    const cleaned = msg.replace(/https?:\/\/[^\s]+/g, "").trim();
    if (cleaned.length < 5) {
      return t("errorGenericSynthesis");
    }

    return cleaned;
  };

  const showAlert = (
    title: string,
    message: string,
    variant: "default" | "destructive" = "default"
  ) => {
    if (alertTimeoutRef.current) clearTimeout(alertTimeoutRef.current);
    const safeMessage = variant === "destructive" ? sanitizeErrorMessage(message) : message;
    setAlertInfo({ title, message: safeMessage, variant });
    alertTimeoutRef.current = setTimeout(() => {
      setAlertInfo(null);
    }, 5000);
  };

  const fetchTelemetry = async () => {
    try {
      const resp = await fetch(`${serverUrl}/health`);
      if (resp.ok) {
        const data = await resp.json();
        setTelemetry(data);
      }
    } catch {
      // Ignora falhas de telemetria
    }
  };

  useEffect(() => {
    fetchTelemetry();
    const timer = setInterval(fetchTelemetry, 6000);
    return () => clearInterval(timer);
  }, [serverUrl]);

  const handleSynthesize = async () => {
    if (!text.trim()) {
      showAlert(t("error"), "Por favor, digite um texto para sintetizar.", "destructive");
      return;
    }

    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    abortControllerRef.current = new AbortController();

    setIsLoading(true);
    setAlertInfo(null);

    const startTime = performance.now();
    let firstByteTime = 0;

    try {
      const formattedPitch = pitchHz >= 0 ? `+${pitchHz}Hz` : `${pitchHz}Hz`;
      let endpoint = `${serverUrl}/v1/audio/speech`;
      let bodyData: any = {
        model: "tts-1",
        input: text.trim(),
        voice,
        response_format: "verbose_json",
        with_subtitles: true,
        speed,
        pitch: formattedPitch,
        break_comma: breakCommaMs > 0 ? `${breakCommaMs}ms` : "",
        break_period: breakPeriodMs > 0 ? `${breakPeriodMs}ms` : "",
        telephony: isTelephony,
        emotion: emotion || undefined,
        emotion_scale: emotion ? emotionScale.toFixed(1) : undefined,
      };

      if (selectedPersonaId) {
        endpoint = `${serverUrl}/v1/persona/${selectedPersonaId}/speech`;
        bodyData = {
          input: text.trim(),
          response_format: "verbose_json",
          with_subtitles: true,
          pitch: formattedPitch,
          break_comma: breakCommaMs > 0 ? `${breakCommaMs}ms` : "",
          break_period: breakPeriodMs > 0 ? `${breakPeriodMs}ms` : "",
          telephony: isTelephony,
          emotion: emotion || undefined,
          emotion_scale: emotion ? emotionScale.toFixed(1) : undefined,
        };
      }

      const resp = await fetch(endpoint, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${apiKey}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(bodyData),
        signal: abortControllerRef.current.signal,
      });

      firstByteTime = performance.now();

      if (!resp.ok) {
        let errMessage = "Falha ao gerar áudio";
        try {
          const errData = await resp.json();
          errMessage = errData.error?.message || errData.message || errMessage;
        } catch {
          // fallback
        }
        throw new Error(errMessage);
      }

      const isCacheHit = resp.headers.get("X-Cache") === "HIT";
      const contentType = resp.headers.get("Content-Type") || "";
      let blob: Blob;

      if (contentType.includes("application/json")) {
        const jsonRes = await resp.json();
        const binary = atob(jsonRes.audio);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        const mimeType = format === "wav" ? "audio/wav" : (format === "opus" ? "audio/ogg" : "audio/mpeg");
        blob = new Blob([bytes], { type: mimeType });
        setSubtitles(jsonRes.subtitles || null);
      } else {
        blob = await resp.blob();
        setSubtitles(null);
      }

      const endTime = performance.now();
      const url = URL.createObjectURL(blob);
      setAudioUrl(url);

      const ttfb = Math.round(firstByteTime - startTime);
      const totalTime = Math.round(endTime - startTime);

      setMetrics({
        ttfb,
        totalTime,
        size: blob.size,
        cacheHit: isCacheHit,
      });

      fetchTelemetry();
    } catch (err: any) {
      if (err.name === "AbortError") return;
      showAlert(t("error"), err.message || "Falha ao gerar áudio", "destructive");
    } finally {
      setIsLoading(false);
    }
  };

  // Atalho de teclado: Ctrl+Enter / Cmd+Enter para sintetizar direto
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
        e.preventDefault();
        handleSynthesize();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [text, voice, format, speed, pitchHz, breakCommaMs, breakPeriodMs, selectedPersonaId]);

  // Identificar Persona ativa e características
  const activePersona = personas.find((p) => p.id === selectedPersonaId);
  const activeVoiceObj = voices.find(
    (v) =>
      v.id.toLowerCase() === voice.toLowerCase() ||
      (v.aliases && v.aliases.some((a) => a.toLowerCase() === voice.toLowerCase()))
  );
  const countryCode = activeVoiceObj?.country_code || "BR";
  const voiceName = activeVoiceObj ? activeVoiceObj.name : voice;

  // Estado dinâmico da Persona para o orbe animado
  const personaState: PersonaState = isLoading
    ? "thinking"
    : isPlayingAudio
    ? "speaking"
    : "idle";

  // Detecção de tags ativas no texto para badges
  const sfxCount = (text.match(/\[(?:som|sfx|sound):\s*[^\]]+\]|\[(?:teclado|callcenter|ruido|ruído|ambiente)(?::\s*[^\]]+)?\]/gi) || []).length;
  const pauseCount = (text.match(/\[(?:pausa|pause|break):\s*[^\]]+\]|\((?:pausa|pause|break):\s*[^)]+\)|<break\s+[^>]*\/?>/gi) || []).length;
  const emotionCount = (text.match(/\[(?:emocao|emoção|emotion):\s*[^\]]+\]|\[(alegre|cheerful|triste|sad|bravo|irritado|angry|animado|empolgado|excited|calmo|calm|gritando|shouting|amigavel|amigável|friendly|esperancoso|esperançoso|hopeful|feliz|happy|serio|sério|firme|surpreso|surprise|tenso|medo|fear)\]/gi) || []).length;
  const hasAmbient = /\[(callcenter|teclado|ruido|ruído|ambiente)\]/i.test(text);

  const isCapCutVoice =
    activeVoiceObj?.provider === "capcut" ||
    voice.toLowerCase().startsWith("dit_") ||
    voice.toLowerCase().startsWith("icl_") ||
    voice.toLowerCase().startsWith("bv") ||
    voice.toLowerCase() === "wenrou" ||
    voice.toLowerCase() === "empresario" ||
    voice.toLowerCase() === "luiz" ||
    voice.toLowerCase() === "youya" ||
    voice.toLowerCase() === "emotional" ||
    voice.toLowerCase() === "dramaaa";

  return (
    <div className="space-y-4">
      {/* Studio Header Bar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pb-2 border-b border-border/60">
        <div>
          <h2 className="text-lg font-bold tracking-tight text-foreground flex items-center gap-2">
            <span>{t("voiceStudioTitle")}</span>
            <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">
              {t("nativeGoBadge")}
            </span>
          </h2>
          <p className="text-xs text-muted-foreground">
            {t("studioHeaderSubtitle")}
          </p>
        </div>

        {/* Top Quick Persona Switcher */}
        <div className="flex items-center gap-2">
          <label className="text-xs text-muted-foreground whitespace-nowrap">
            Persona:
          </label>
          <div className="relative">
            <select
              value={selectedPersonaId}
              onChange={(e) => applyPersona(e.target.value)}
              className="h-8 pl-2.5 pr-7 rounded-md border border-border/80 bg-background text-xs text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary appearance-none cursor-pointer transition-colors"
            >
              <option value="">{t("noneCustom")}</option>
              {personas.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
            <IonIcon
              name="chevron-down-outline"
              className="absolute right-2 top-2.5 text-[10px] text-muted-foreground pointer-events-none"
            />
          </div>
        </div>
      </div>

      {alertInfo && (
        <Alert
          variant={alertInfo.variant}
          className="animate-in fade-in slide-in-from-top-2 duration-200"
        >
          <IonIcon
            name={
              alertInfo.variant === "destructive"
                ? "alert-circle-outline"
                : "checkmark-circle-outline"
            }
            className="h-4 w-4"
          />
          <AlertTitle className="text-xs font-semibold">
            {alertInfo.title}
          </AlertTitle>
          <AlertDescription className="text-xs">
            {alertInfo.message}
          </AlertDescription>
        </Alert>
      )}

      {/* Main Studio 2-Column Workstation */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
        {/* Left Column: Studio Canvas (7 cols on lg, 8 cols on xl) */}
        <div className="lg:col-span-7 xl:col-span-8 space-y-3.5">
          {/* Textarea Studio Canvas */}
          <div className="rounded-xl border border-border/80 bg-card/60 backdrop-blur-sm overflow-hidden shadow-sm transition-all focus-within:ring-1 focus-within:ring-primary focus-within:border-primary">
            <TagEditorToolbar
              textareaRef={textareaRef}
              text={text}
              setText={setText}
              voices={voices}
              isCapCutVoice={isCapCutVoice}
            />

            <div className="p-3">
              <Textarea
                ref={textareaRef}
                value={text}
                onChange={(e) => setText(e.target.value)}
                rows={7}
                placeholder={t("textInputPlaceholder")}
                className="w-full resize-none font-sans text-sm border-0 rounded-none focus-visible:ring-0 p-0 bg-transparent leading-relaxed"
              />
            </div>

            {/* Footer do Canvas com Contadores de Tags e Atalho */}
            <div className="px-3 py-2 bg-secondary/30 border-t border-border/50 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-mono text-[11px]">
                  {text.length} {t("charCount")}
                </span>

                {hasAmbient && (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/30 text-[10px] font-mono">
                    <IonIcon name="layers-outline" className="text-xs" />
                    <span>{t("activeAmbientBadge")}</span>
                  </span>
                )}

                {sfxCount > 0 && (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/30 text-[10px] font-mono">
                    <IonIcon name="volume-medium-outline" className="text-xs" />
                    <span>{sfxCount}x {t("sfxBadge")}</span>
                  </span>
                )}

                {pauseCount > 0 && (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-zinc-500/10 text-zinc-300 border border-zinc-500/30 text-[10px] font-mono">
                    <IonIcon name="timer-outline" className="text-xs" />
                    <span>{pauseCount}x {t("pausesBadge")}</span>
                  </span>
                )}

                {emotionCount > 0 && (
                  <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-400 border border-purple-500/30 text-[10px] font-mono">
                    <IonIcon name="sparkles-outline" className="text-xs" />
                    <span>{emotionCount}x {t("emotionBadge")}</span>
                  </span>
                )}
              </div>

              <span className="text-[10px] font-mono text-muted-foreground hidden sm:inline">
                {t("ctrlEnterHint")}
              </span>
            </div>
          </div>

          {/* Action Button: Sintetizar */}
          <Button
            onClick={handleSynthesize}
            disabled={isLoading || !text.trim()}
            className="w-full h-10 text-xs font-semibold shadow-sm transition-all"
          >
            {isLoading ? (
              <>
                <IonIcon name="sync-outline" className="text-sm animate-spin mr-2" />
                <span>{t("synthesizingWait")}</span>
              </>
            ) : (
              <>
                <IonIcon name="volume-high-outline" className="text-sm mr-2" />
                <span>{t("synthesizeButton")}</span>
              </>
            )}
          </Button>

          {/* Integrated Waveform Player (Aparece diretamente sob o botão quando gerado) */}
          {audioUrl && !isLoading && (
            <div className="rounded-xl border border-border/80 bg-card/60 backdrop-blur-sm p-4 shadow-sm animate-in fade-in slide-in-from-top-2 duration-300">
              <div className="flex items-center justify-between mb-3 text-xs">
                <span className="font-semibold text-foreground flex items-center gap-1.5">
                  <IonIcon name="musical-notes-outline" className="text-primary text-xs" />
                  <span>{t("generatedAudioTitle")}</span>
                </span>
                {metrics && (
                  <div className="flex items-center gap-2 text-[10px] font-mono text-muted-foreground">
                    <span>TTFB: {metrics.ttfb}ms</span>
                    <span>•</span>
                    <span>{(metrics.totalTime / 1000).toFixed(2)}s</span>
                    <span>•</span>
                    <span>{formatBytes(metrics.size)}</span>
                  </div>
                )}
              </div>
              <WaveformPlayer
                audioUrl={audioUrl}
                format={format}
                personaId={selectedPersonaId}
                subtitles={subtitles}
                metrics={metrics}
                onPlayStateChange={setIsPlayingAudio}
              />
            </div>
          )}
        </div>

        {/* Right Column: Studio Voice & Persona Inspector (5 cols on lg, 4 cols on xl) */}
        <div className="lg:col-span-5 xl:col-span-4 space-y-4">
          <Card className="p-4 bg-card/60 border-border/80 shadow-none space-y-4">
            {/* 1. Persona Identity & Animated AI Elements Orb */}
            <div className="flex items-center gap-3 pb-3 border-b border-border/60">
              <div className="relative group shrink-0">
                <div className="w-12 h-12 rounded-full flex items-center justify-center bg-zinc-950 border border-border shadow-inner overflow-hidden">
                  <AiPersona
                    state={personaState}
                    variant="obsidian"
                    className="w-12 h-12"
                  />
                </div>
                <span
                  className={`absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full border-2 border-background transition-colors ${
                    personaState === "speaking"
                      ? "bg-emerald-400 animate-ping"
                      : personaState === "thinking"
                      ? "bg-amber-400"
                      : "bg-emerald-500"
                  }`}
                />
              </div>

              <div className="space-y-0.5 min-w-0 flex-1">
                <div className="flex items-center justify-between gap-1">
                  <h4 className="font-semibold text-sm text-foreground truncate">
                    {activePersona ? activePersona.name : t("freeVoice")}
                  </h4>
                  <span
                    className={`text-[9px] font-mono px-1.5 py-0.2 rounded border ${
                      personaState === "speaking"
                        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                        : personaState === "thinking"
                        ? "bg-amber-500/10 text-amber-400 border-amber-500/20"
                        : "bg-secondary text-muted-foreground border-border/40"
                    }`}
                  >
                    {personaState === "speaking"
                      ? t("statusPlaying")
                      : personaState === "thinking"
                      ? t("statusSynthesizing")
                      : t("statusReady")}
                  </span>
                </div>
                <p className="text-[11px] text-muted-foreground truncate">
                  {activePersona?.description || t("manualConfigDesc")}
                </p>
              </div>
            </div>

            {/* 2. Voice Selection */}
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground">
                {t("neuralVoiceLabel")}
              </label>
              <VoiceSelect
                voices={voices}
                value={voice}
                onChange={(newVoice) => setVoice(newVoice)}
              />
            </div>

            {/* Bloco Dinâmico: Sentimentos & Expressividade CapCut */}
            {isCapCutVoice && (
              <div className="p-3 bg-gradient-to-br from-purple-950/40 via-purple-900/10 to-background/50 rounded-xl border border-purple-500/30 space-y-2.5 animate-in fade-in slide-in-from-top-2 duration-200 shadow-sm">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold text-purple-200 flex items-center gap-1.5">
                    <IonIcon name="sparkles-outline" className="text-purple-400 text-xs" />
                    <span>{t("capcutEmotionsTitle")}</span>
                  </span>
                  <span className="text-[9px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-300 border border-purple-500/30 font-medium">
                    {t("nativeByteDanceBadge")}
                  </span>
                </div>

                {/* Card de Personalidade Nativa da Voz */}
                {activeVoiceObj?.personality && (
                  <div className="p-2 rounded-lg bg-purple-950/50 border border-purple-500/20 text-[11px] text-purple-200/90 leading-relaxed">
                    <span className="font-semibold text-purple-300 block mb-0.5">
                      {t("nativePersonalityTitle")}
                    </span>
                    {activeVoiceObj.personality}
                  </div>
                )}

                {/* Seletor de Emoção / Sentimento em Pills */}
                <div className="space-y-1.5">
                  <div className="flex justify-between text-[11px] text-muted-foreground">
                    <span>{t("emotionSelectorLabel")}</span>
                    <span className="font-mono text-purple-300 font-medium capitalize">
                      {emotion || t("naturalEmotion")}
                    </span>
                  </div>
                  <div className="grid grid-cols-4 gap-1">
                    {[
                      { id: "", label: t("naturalEmotion"), icon: "😐" },
                      { id: "calm", label: t("calmEmotion"), icon: "😌" },
                      { id: "happy", label: t("happyEmotion"), icon: "😊" },
                      { id: "angry", label: t("firmEmotion"), icon: "😠" },
                      { id: "sad", label: t("sadEmotion"), icon: "😢" },
                      { id: "surprise", label: t("surpriseEmotion"), icon: "😲" },
                      { id: "fear", label: t("fearEmotion"), icon: "😱" },
                    ].map((em) => {
                      const isSelected = emotion === em.id;
                      return (
                        <button
                          key={em.id}
                          type="button"
                          onClick={() => setEmotion(em.id)}
                          className={`flex items-center justify-center gap-1 py-1.5 px-1 rounded-md text-[11px] transition-all ${
                            isSelected
                              ? "bg-purple-500 text-white shadow-sm ring-1 ring-purple-400 font-semibold"
                              : "bg-secondary/60 text-muted-foreground hover:text-foreground hover:bg-secondary"
                          }`}
                        >
                          <span>{em.icon}</span>
                          <span className="truncate">{em.label}</span>
                        </button>
                      );
                    })}
                  </div>
                </div>

                {/* Slider de Intensidade do Sentimento (Emotion Scale) */}
                {emotion && (
                  <div className="space-y-1 pt-1 animate-in fade-in duration-150">
                    <div className="flex justify-between text-[11px] text-muted-foreground">
                      <span>{t("emotionIntensityLabel")}</span>
                      <span className="font-mono text-purple-300 font-semibold">
                        {emotionScale.toFixed(1)}x{" "}
                        {emotionScale <= 0.6 ? t("intensitySubtle") : emotionScale <= 1.2 ? t("intensityBalanced") : t("intensityIntense")}
                      </span>
                    </div>
                    <input
                      type="range"
                      min="0.2"
                      max="2.0"
                      step="0.1"
                      value={emotionScale}
                      onChange={(e) => setEmotionScale(parseFloat(e.target.value))}
                      className="w-full accent-purple-500 h-1.5 bg-secondary rounded-lg cursor-pointer"
                    />
                  </div>
                )}
              </div>
            )}

            {/* 3. Sliders de Velocidade e Tom */}
            <div className="space-y-3 pt-1">
              <div>
                <div className="flex justify-between text-xs text-muted-foreground mb-1">
                  <span>{t("speedLabel")}</span>
                  <span className="font-mono text-foreground font-medium">
                    {speed.toFixed(2)}x
                  </span>
                </div>
                <input
                  type="range"
                  min="0.5"
                  max="2.0"
                  step="0.05"
                  value={speed}
                  onChange={(e) => setSpeed(parseFloat(e.target.value))}
                  className="w-full accent-primary h-1.5 bg-secondary rounded-lg cursor-pointer"
                />
              </div>

              <div>
                <div className="flex justify-between text-xs text-muted-foreground mb-1">
                  <span>{t("pitchLabel")}</span>
                  <span className="font-mono text-foreground font-medium">
                    {pitchHz >= 0 ? `+${pitchHz}Hz` : `${pitchHz}Hz`}
                  </span>
                </div>
                <input
                  type="range"
                  min="-20"
                  max="20"
                  step="1"
                  value={pitchHz}
                  onChange={(e) => setPitchHz(parseInt(e.target.value, 10))}
                  className="w-full accent-primary h-1.5 bg-secondary rounded-lg cursor-pointer"
                />
              </div>
            </div>

            {/* 4. Pausas & Expressividade Natural (SSML) */}
            <div className="p-2.5 bg-secondary/30 rounded-lg border border-border/70 space-y-2.5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-foreground flex items-center gap-1.5">
                  <IonIcon name="timer-outline" className="text-primary text-xs" />
                  <span>{t("ssmlBreakSectionTitle")}</span>
                </span>
                <span className="text-[9px] px-1 py-0.5 rounded bg-emerald-500/10 text-emerald-400 font-mono">
                  {t("costZeroBadge")}
                </span>
              </div>

              <div className="space-y-2">
                <div>
                  <div className="flex justify-between text-[11px] text-muted-foreground mb-0.5">
                    <span>{t("breakCommaLabel")}</span>
                    <span className="font-mono text-emerald-400 font-medium">
                      {breakCommaMs}ms
                    </span>
                  </div>
                  <input
                    type="range"
                    min="0"
                    max="800"
                    step="25"
                    value={breakCommaMs}
                    onChange={(e) => setBreakCommaMs(parseInt(e.target.value, 10))}
                    className="w-full accent-emerald-500 cursor-pointer h-1.5 bg-zinc-800 rounded-lg appearance-none"
                  />
                </div>

                <div>
                  <div className="flex justify-between text-[11px] text-muted-foreground mb-0.5">
                    <span>{t("breakPeriodLabel")}</span>
                    <span className="font-mono text-emerald-400 font-medium">
                      {breakPeriodMs}ms
                    </span>
                  </div>
                  <input
                    type="range"
                    min="0"
                    max="1200"
                    step="50"
                    value={breakPeriodMs}
                    onChange={(e) => setBreakPeriodMs(parseInt(e.target.value, 10))}
                    className="w-full accent-emerald-500 cursor-pointer h-1.5 bg-zinc-800 rounded-lg appearance-none"
                  />
                </div>
              </div>
            </div>

            {/* 5. Realismo Telefônico & Respiração Humana (DSP) */}
            <div className="p-2.5 bg-secondary/30 rounded-lg border border-border/70 space-y-2.5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium text-foreground flex items-center gap-1.5">
                  <IonIcon name="call-outline" className="text-blue-400 text-xs" />
                  <span>{t("telephonyRealismTitle")}</span>
                </span>
                <span className="text-[9px] px-1 py-0.5 rounded bg-blue-500/10 text-blue-400 font-mono">
                  DSP 3.4kHz
                </span>
              </div>

              <div className="space-y-2">
                <label className="flex items-center justify-between cursor-pointer group">
                  <div className="space-y-0.5 pr-2">
                    <span className="text-xs text-foreground font-medium block group-hover:text-blue-400 transition-colors">
                      {t("telephonyFilterLabel")}
                    </span>
                    <span className="text-[10px] text-muted-foreground block">
                      {t("telephonyFilterDesc")}
                    </span>
                  </div>
                  <input
                    type="checkbox"
                    checked={isTelephony}
                    onChange={(e) => setIsTelephony(e.target.checked)}
                    className="h-4 w-4 rounded border-border bg-background text-primary accent-primary cursor-pointer"
                  />
                </label>
              </div>
            </div>

            {/* 6. Formato de Saída */}
            <div className="space-y-1">
              <label className="text-xs font-medium text-muted-foreground">
                {t("outputFormatLabel")}
              </label>
              <select
                value={format}
                onChange={(e) => setFormat(e.target.value)}
                className="flex h-8 w-full rounded-md border border-input bg-background px-2.5 py-1 text-xs text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                <option value="mp3">MP3 (24kHz 48kbps Mono)</option>
                <option value="wav">WAV (PCM 16-bit 24kHz)</option>
                <option value="opus">Opus (Ogg Container 24kHz)</option>
                <option value="aac">AAC (ADTS 24kHz)</option>
              </select>
            </div>

            {/* 6. Discreta Telemetria do Servidor em Go */}
            <div className="pt-2 border-t border-border/50 grid grid-cols-2 gap-2 text-[10px] font-mono text-muted-foreground">
              <div className="flex items-center justify-between">
                <span>{t("telemetryLatency")}</span>
                <span className="text-foreground">{metrics ? `${metrics.ttfb}ms` : "--"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>{t("telemetryGoroutines")}</span>
                <span className="text-foreground flex items-center gap-1">
                  <span>{telemetry ? telemetry.goroutines : "--"}</span>
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span>{t("telemetryRAM")}</span>
                <span className="text-foreground">
                  {telemetry ? `${telemetry.alloc_mb.toFixed(1)}MB` : "--"}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span>{t("telemetryCache")}</span>
                <span className="text-foreground">
                  {telemetry ? `${telemetry.cache_items} ${t("telemetryItems")}` : "--"}
                </span>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}
