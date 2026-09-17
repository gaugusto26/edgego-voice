"use client";

import React, { useState, useRef, useEffect, useMemo, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { IonIcon } from "@/components/ui/ion-icon";
import { SubtitleResult } from "@/types";
import { useLanguage } from "@/i18n/LanguageContext";

interface WaveformPlayerProps {
  audioUrl: string;
  format: string;
  personaId?: string;
  subtitles?: SubtitleResult | null;
  metrics: {
    ttfb: number;
    totalTime: number;
    size: number;
    cacheHit?: boolean;
  } | null;
  onPlayStateChange?: (isPlaying: boolean) => void;
}

// SUBTITLE_LEAD_MS: compensação de latência do hardware de áudio e do pipeline de render do navegador.
// O audio.currentTime reflete a posição do decodificador, não a posição real dos alto-falantes.
// 60ms = ~30ms de latência do driver de áudio + ~16ms de frame do rAF + ~14ms de margem.
const SUBTITLE_LEAD_MS = 60;

export function WaveformPlayer({ audioUrl, format, personaId, subtitles, onPlayStateChange }: WaveformPlayerProps) {
  const { t } = useLanguage();
  const [isPlaying, setIsPlaying] = useState(false);

  useEffect(() => {
    onPlayStateChange?.(isPlaying);
  }, [isPlaying, onPlayStateChange]);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [playbackRate, setPlaybackRate] = useState(1.0);
  const [isMuted, setIsMuted] = useState(false);
  const [hoverPct, setHoverPct] = useState<number | null>(null);
  const [needsReload, setNeedsReload] = useState(false);
  const [activeCueIndex, setActiveCueIndex] = useState<number | null>(null);
  const [showSubtitlesDrawer, setShowSubtitlesDrawer] = useState(false);
  const [copiedJSON, setCopiedJSON] = useState(false);

  const audioRef = useRef<HTMLAudioElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const rafRef = useRef<number | null>(null);
  // Ref para evitar stale closure na sentença ativa
  const subtitlesRef = useRef(subtitles);

  const activeCue = useMemo(() => {
    if (!subtitles?.sentences || activeCueIndex === null) return null;
    return subtitles.sentences.find((c) => c.index === activeCueIndex) || null;
  }, [subtitles, activeCueIndex]);

  const peaks = useMemo(() => {
    const arr: number[] = [];
    for (let i = 0; i < 54; i++) {
      const s1 = Math.sin((i / 54) * Math.PI) * 0.65;
      const s2 = Math.sin(i * 0.75) * 0.22;
      const s3 = Math.cos(i * 1.4) * 0.13;
      arr.push(Math.max(0.18, Math.min(0.95, Math.abs(s1 + s2 + s3))));
    }
    return arr;
  }, []);

  // Manter subtitlesRef sincronizado (evita stale closure no rAF)
  useEffect(() => {
    subtitlesRef.current = subtitles;
  }, [subtitles]);

  // Loop rAF: atualiza cue ativo a 60 fps enquanto o áudio toca.
  // Aplica SUBTITLE_LEAD_MS de lookahead para compensar a latência do hardware
  // de áudio e o tempo de render do navegador (rAF → DOM → pintura).
  const startRaf = useCallback(() => {
    const tick = () => {
      const audio = audioRef.current;
      if (!audio) return;
      // curMs com lookahead: compensa latência acústica e render
      const curMs = audio.currentTime * 1000 + SUBTITLE_LEAD_MS;
      const sents = subtitlesRef.current?.sentences;
      if (sents && sents.length > 0) {
        let activeFound: (typeof sents)[0] | null = null;
        for (let i = 0; i < sents.length; i++) {
          const cue = sents[i];
          const nextCue = sents[i + 1];

          if (curMs >= cue.start_ms) {
            // Se não há próxima sentença, permanece ativa até o final do áudio
            if (!nextCue) {
              activeFound = cue;
              break;
            }
            // Se ainda não começou a próxima sentença:
            if (curMs < nextCue.start_ms) {
              // Em silêncios propositais longos (> 1.2s), limpa após 600ms do término da frase
              const pauseGap = nextCue.start_ms - cue.end_ms;
              if (pauseGap > 1200 && curMs > cue.end_ms + 600) {
                activeFound = null;
              } else {
                activeFound = cue;
              }
              break;
            }
          }
        }
        setActiveCueIndex(activeFound ? activeFound.index : null);
      }

      rafRef.current = requestAnimationFrame(tick);
    };
    rafRef.current = requestAnimationFrame(tick);
  }, []);

  const stopRaf = useCallback(() => {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
  }, []);

  // Toca automaticamente quando audioUrl muda
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !audioUrl) return;

    stopRaf();
    setCurrentTime(0);
    setDuration(0);
    setIsPlaying(false);
    setNeedsReload(false);
    setActiveCueIndex(null);

    // load() + play() na nova URL
    audio.load();
    audio.play()
      .then(() => setIsPlaying(true))
      .catch(() => setIsPlaying(false));

    return () => stopRaf();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [audioUrl]);

  // Iniciar/parar rAF conforme estado de reprodução
  useEffect(() => {
    if (isPlaying) {
      startRaf();
    } else {
      stopRaf();
    }
    return () => stopRaf();
  }, [isPlaying, startRaf, stopRaf]);

  const handleLoadedMetadata = () => {
    const audio = audioRef.current;
    if (audio && isFinite(audio.duration) && audio.duration > 0) {
      setDuration(audio.duration);
    }
  };

  const handleTimeUpdate = () => {
    const audio = audioRef.current;
    if (!audio) return;
    // Apenas atualiza progresso visual e duração — cue ativo é gerido pelo rAF
    setCurrentTime(audio.currentTime || 0);
    if (!duration && audio.duration && isFinite(audio.duration) && audio.duration > 0) {
      setDuration(audio.duration);
    }
  };

  const handleEnded = () => {
    stopRaf();
    setIsPlaying(false);
    setCurrentTime(0);
    setActiveCueIndex(null);
    // Marcar que precisa de reload antes do próximo play
    setNeedsReload(true);
  };

  const downloadFile = (content: string, filename: string, type: string) => {
    const blob = new Blob([content], { type });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const handleDownloadSRT = () => {
    if (!subtitles?.srt) return;
    downloadFile(subtitles.srt, `subtitles_${personaId || "speech"}.srt`, "application/x-subrip;charset=utf-8");
  };

  const handleDownloadVTT = () => {
    if (!subtitles?.vtt) return;
    downloadFile(subtitles.vtt, `subtitles_${personaId || "speech"}.vtt`, "text/vtt;charset=utf-8");
  };

  const handleCopyJSON = () => {
    if (!subtitles) return;
    navigator.clipboard.writeText(JSON.stringify(subtitles, null, 2));
    setCopiedJSON(true);
    setTimeout(() => setCopiedJSON(false), 2500);
  };

  const handleSeekToCue = (startMs: number) => {
    const audio = audioRef.current;
    if (!audio) return;
    const targetSec = startMs / 1000;
    audio.currentTime = targetSec;
    setCurrentTime(targetSec);
    if (!isPlaying) {
      doPlay(targetSec);
    }
  };

  const doPlay = async (offset?: number) => {
    const audio = audioRef.current;
    if (!audio) return;

    if (needsReload) {
      // Ao terminar o audio, o browser expira a stream do Blob.
      // load() reinicializa o decoder interno sem re-baixar o arquivo (o Blob ainda existe).
      audio.load();
      setNeedsReload(false);
      // Aguardar o browser estar pronto (canplay)
      await new Promise<void>((resolve) => {
        const onReady = () => {
          audio.removeEventListener("canplay", onReady);
          resolve();
        };
        audio.addEventListener("canplay", onReady);
      });
    }

    if (offset !== undefined) {
      audio.currentTime = offset;
    } else if (audio.ended || audio.currentTime >= (audio.duration || duration)) {
      audio.currentTime = 0;
    }

    try {
      await audio.play();
      setIsPlaying(true);
    } catch {
      // Último fallback
      audio.currentTime = 0;
      audio.play().then(() => setIsPlaying(true)).catch(() => {});
    }
  };

  const togglePlayPause = () => {
    const audio = audioRef.current;
    if (!audio) return;

    if (isPlaying) {
      audio.pause();
      setIsPlaying(false);
    } else {
      doPlay();
    }
  };

  const restartAudio = async () => {
    const audio = audioRef.current;
    if (!audio) return;

    audio.pause();
    setCurrentTime(0);
    setNeedsReload(false);

    // Sempre reinicializa o decoder para garantir que o restart funcione
    // tanto durante a reprodução quanto após o término
    audio.load();

    await new Promise<void>((resolve) => {
      const onReady = () => {
        audio.removeEventListener("canplay", onReady);
        resolve();
      };
      audio.addEventListener("canplay", onReady);
    });

    audio.play().then(() => setIsPlaying(true)).catch(() => {});
  };

  const seekTo = (pct: number) => {
    const audio = audioRef.current;
    if (!audio) return;
    const totalDur = duration > 0 ? duration : (audio.duration && isFinite(audio.duration) ? audio.duration : 1);
    const targetTime = Math.max(0, Math.min(1, pct)) * totalDur;
    setCurrentTime(targetTime);
    doPlay(targetTime);
  };

  const handleContainerClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    seekTo((e.clientX - rect.left) / rect.width);
  };

  const handleMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    setHoverPct(Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width)));
  };

  const toggleMute = () => {
    const audio = audioRef.current;
    if (!audio) return;
    audio.muted = !isMuted;
    setIsMuted(!isMuted);
  };

  const changeSpeed = () => {
    const audio = audioRef.current;
    if (!audio) return;
    const speeds = [1.0, 1.25, 1.5, 2.0];
    const next = speeds[(speeds.indexOf(playbackRate) + 1) % speeds.length];
    audio.playbackRate = next;
    setPlaybackRate(next);
  };

  const formatTime = (secs: number) => {
    if (isNaN(secs) || !isFinite(secs) || secs < 0) return "0:00";
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    return `${m}:${s < 10 ? "0" : ""}${s}`;
  };

  // Formato com décimo de segundo para legendas (ex: 0:01.4) — mais preciso que formatTime
  const formatCueTime = (secs: number) => {
    if (isNaN(secs) || !isFinite(secs) || secs < 0) return "0:00.0";
    const m = Math.floor(secs / 60);
    const s = Math.floor(secs % 60);
    const ds = Math.floor((secs % 1) * 10); // décimo de segundo
    return `${m}:${s < 10 ? "0" : ""}${s}.${ds}`;
  };

  const currentDuration = duration > 0 ? duration : (audioRef.current?.duration && isFinite(audioRef.current.duration) ? audioRef.current.duration : 0);
  const progressPct = currentDuration > 0 ? Math.min(100, (currentTime / currentDuration) * 100) : 0;

  return (
    <div className="w-full space-y-3 pt-1 animate-card-enter">
      <audio
        ref={audioRef}
        src={audioUrl}
        preload="auto"
        onTimeUpdate={handleTimeUpdate}
        onLoadedMetadata={handleLoadedMetadata}
        onEnded={handleEnded}
        onPlay={() => setIsPlaying(true)}
        onPause={() => setIsPlaying(false)}
        className="hidden"
      />

      {/* Waveform Visualizer */}
      <div
        ref={containerRef}
        onClick={handleContainerClick}
        onMouseMove={handleMouseMove}
        onMouseLeave={() => setHoverPct(null)}
        className="relative w-full h-24 bg-[#09090b]/80 border border-border/70 rounded-xl p-3.5 flex items-center justify-between gap-[3px] cursor-pointer select-none group transition-colors hover:border-zinc-700 overflow-hidden"
      >
        {hoverPct !== null && currentDuration > 0 && (
          <div
            className="absolute top-1 bottom-1 w-[1.5px] bg-foreground/60 pointer-events-none z-10"
            style={{ left: `${hoverPct * 100}%` }}
          >
            <span className="absolute -top-1 left-1/2 -translate-x-1/2 -translate-y-full text-[9px] font-mono bg-zinc-900 border border-border px-1.5 py-0.5 rounded text-foreground shadow-md">
              {formatTime(hoverPct * currentDuration)}
            </span>
          </div>
        )}

        <div
          className="absolute top-0 bottom-0 w-[2px] bg-primary z-10 pointer-events-none transition-all duration-75 shadow-[0_0_8px_rgba(0,223,129,0.5)]"
          style={{ left: `${progressPct}%` }}
        />

        {peaks.map((amplitude, idx) => {
          const barPct = (idx / peaks.length) * 100;
          const isPassed = barPct <= progressPct;
          return (
            <div key={idx} className="flex-1 flex items-center justify-center h-full pointer-events-none">
              <div
                className={`w-full max-w-[3px] rounded-full transition-all duration-100 ${
                  isPassed
                    ? "bg-primary shadow-[0_0_4px_rgba(0,223,129,0.4)]"
                    : "bg-[#27272a] group-hover:bg-[#323238]"
                }`}
                style={{ height: `${Math.max(12, amplitude * 85)}%` }}
              />
            </div>
          );
        })}
      </div>

      {/* Controls */}
      <div className="flex items-center justify-between px-1">
        <div className="flex items-center gap-2">
          <Button variant="primary" size="icon" onClick={togglePlayPause} className="h-8 w-8 rounded-full shadow-sm" title={isPlaying ? t("playerPause") : t("playerPlay")}>
            <IonIcon name={isPlaying ? "pause-outline" : "play-outline"} className="text-base" />
          </Button>

          <Button variant="ghost" size="icon" onClick={restartAudio} className="h-8 w-8 rounded-full text-muted-foreground hover:text-foreground" title={t("playerRestart")}>
            <IonIcon name="reload-outline" className="text-sm" />
          </Button>

          <button onClick={changeSpeed} className="text-[11px] font-mono font-medium text-muted-foreground hover:text-foreground bg-secondary px-2 py-0.5 rounded transition-colors">
            {playbackRate}x
          </button>
        </div>

        <div className="text-xs font-mono text-muted-foreground select-none">
          <span className="text-foreground font-semibold">{formatTime(currentTime)}</span>
          <span className="mx-1">/</span>
          <span>{formatTime(currentDuration)}</span>
        </div>

        <div className="flex items-center gap-1.5">
          <Button variant="ghost" size="icon" onClick={toggleMute} className="h-8 w-8 rounded-full text-muted-foreground hover:text-foreground" title={isMuted ? t("playerUnmute") : t("playerMute")}>
            <IonIcon name={isMuted ? "volume-mute-outline" : "volume-high-outline"} className="text-sm" />
          </Button>

          <a
            href={audioUrl}
            download={`audio_${personaId || "speech"}.${format}`}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-input bg-background hover:bg-accent text-xs font-medium text-foreground transition-colors shadow-sm"
          >
            <IonIcon name="download-outline" className="text-sm text-primary" />
            <span>{t("playerDownload")} {format.toUpperCase()}</span>
          </a>
        </div>
      </div>

      {/* Subtitles & Karaoke Synchronized Section */}
      {subtitles && (
        <div className="rounded-xl border border-border/80 bg-zinc-950/60 p-3.5 space-y-3 mt-2 shadow-sm animate-in fade-in duration-200">
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
            <div className="flex items-center gap-2">
              <span className="flex items-center gap-1.5 font-semibold text-foreground">
                <IonIcon name="chatbubble-ellipses-outline" className="text-primary text-sm" />
                <span>{t("subtitlesLabel")}</span>
              </span>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-primary/10 text-primary border border-primary/20">
                {subtitles.sentences?.length || 0} {t("subtitlesSentences")}
              </span>
            </div>

            {/* Quick Export Subtitles Actions */}
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={handleDownloadSRT}
                className="inline-flex items-center gap-1 px-2 py-1 rounded bg-secondary/80 hover:bg-secondary text-[11px] font-medium text-foreground transition-all hover:border-border border border-transparent shadow-2xs"
                title={t("subtitlesDownloadSRT")}
              >
                <IonIcon name="document-text-outline" className="text-amber-400 text-xs" />
                <span>.SRT</span>
              </button>

              <button
                type="button"
                onClick={handleDownloadVTT}
                className="inline-flex items-center gap-1 px-2 py-1 rounded bg-secondary/80 hover:bg-secondary text-[11px] font-medium text-foreground transition-all hover:border-border border border-transparent shadow-2xs"
                title={t("subtitlesDownloadVTT")}
              >
                <IonIcon name="videocam-outline" className="text-blue-400 text-xs" />
                <span>.VTT</span>
              </button>

              <button
                type="button"
                onClick={handleCopyJSON}
                className="inline-flex items-center gap-1 px-2 py-1 rounded bg-secondary/80 hover:bg-secondary text-[11px] font-medium text-foreground transition-all hover:border-border border border-transparent shadow-2xs"
                title={t("subtitlesCopyJSON")}
              >
                <IonIcon
                  name={copiedJSON ? "checkmark-outline" : "copy-outline"}
                  className={`text-xs ${copiedJSON ? "text-emerald-400" : "text-purple-400"}`}
                />
                <span>{copiedJSON ? t("subtitlesCopied") : "JSON"}</span>
              </button>
            </div>
          </div>

          {/* Active Live Sentence Callout Box */}
          <div className="min-h-[44px] flex items-center justify-center text-center px-4 py-2 rounded-lg bg-zinc-900/80 border border-border/50">
            {activeCue ? (
              <div className="space-y-0.5 animate-in fade-in duration-150">
                <p className="text-xs sm:text-sm font-semibold text-foreground tracking-wide leading-relaxed">
                  "{activeCue.text}"
                </p>
                <p className="text-[10px] font-mono text-primary/80">
                  {formatCueTime(activeCue.start_ms / 1000)} – {formatCueTime(activeCue.end_ms / 1000)}
                </p>
              </div>
            ) : (
              <span className="text-xs text-muted-foreground italic flex items-center gap-1.5">
                <IonIcon name="play-circle-outline" className="text-xs opacity-70" />
                <span>{isPlaying ? t("subtitlesWaitingNext") : t("subtitlesPlayPrompt")}</span>
              </span>
            )}
          </div>

          {/* Toggle Accordion to view & navigate all sentence cues */}
          {subtitles.sentences && subtitles.sentences.length > 0 && (
            <div className="pt-1">
              <button
                type="button"
                onClick={() => setShowSubtitlesDrawer(!showSubtitlesDrawer)}
                className="text-[11px] text-muted-foreground hover:text-foreground flex items-center gap-1 transition-colors"
              >
                <IonIcon
                  name={showSubtitlesDrawer ? "chevron-up-outline" : "chevron-down-outline"}
                  className="text-xs"
                />
                <span>{showSubtitlesDrawer ? t("subtitlesHideList") : t("subtitlesShowList")}</span>
              </button>

              {showSubtitlesDrawer && (
                <div className="max-h-48 overflow-y-auto space-y-1 mt-2 pr-1 custom-scrollbar text-xs animate-in fade-in duration-150">
                  {subtitles.sentences.map((cue) => {
                    const isActive = cue.index === activeCueIndex;
                    return (
                      <button
                        key={cue.index}
                        type="button"
                        onClick={() => handleSeekToCue(cue.start_ms)}
                        className={`w-full text-left p-2 rounded-md transition-all flex items-start gap-2.5 ${
                          isActive
                            ? "bg-primary/20 border border-primary/40 text-primary font-semibold shadow-xs"
                            : "bg-secondary/40 hover:bg-secondary text-muted-foreground hover:text-foreground border border-transparent"
                        }`}
                        title={t("subtitlesSeekTitle")}
                      >
                        <span className="font-mono text-[10px] shrink-0 opacity-80 mt-0.5 bg-background/50 px-1.5 py-0.5 rounded">
                          {formatCueTime(cue.start_ms / 1000)}
                        </span>
                        <span className="flex-1 leading-relaxed text-left">{cue.text}</span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
