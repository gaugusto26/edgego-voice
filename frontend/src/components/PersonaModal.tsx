"use client";

import React, { useState, useEffect } from "react";
import { Persona, VoiceOption } from "@/types";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogContent,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldTitle,
} from "@/components/ui/field";
import { IonIcon } from "@/components/ui/ion-icon";
import { VoiceSelect } from "@/components/ui/voice-select";
import { useLanguage } from "@/i18n/LanguageContext";

interface PersonaModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingPersona: Persona | null;
  voices: VoiceOption[];
  onSave: (personaData: Partial<Persona>) => Promise<void>;
}

export function PersonaModal({
  open,
  onOpenChange,
  editingPersona,
  voices,
  onSave,
}: PersonaModalProps) {
  const { t } = useLanguage();
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [voice, setVoice] = useState("pt-BR-ThalitaMultilingualNeural");
  const [format, setFormat] = useState("mp3");
  const [speed, setSpeed] = useState(1.0);
  const [pitchHz, setPitchHz] = useState(0);
  const [breakCommaMs, setBreakCommaMs] = useState(150);
  const [breakPeriodMs, setBreakPeriodMs] = useState(350);
  const [telephony, setTelephony] = useState(false);
  const [sanitize, setSanitize] = useState(true);
  const [emotion, setEmotion] = useState("");
  const [emotionScale, setEmotionScale] = useState(1.0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isIdCustomized, setIsIdCustomized] = useState(false);

  // Função para sanitizar e transformar texto em slug seguro
  const slugify = (text: string) => {
    return text
      .toLowerCase()
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "") // remove acentuações
      .replace(/[^a-z0-9\s-_]/g, "") // remove caracteres especiais
      .replace(/[\s_]+/g, "-") // converte espaços para hífens
      .replace(/-+/g, "-"); // colapsa múltiplos hífens
  };

  const parseHz = (p?: string) => {
    if (!p) return 0;
    const num = parseInt(p.replace(/[^0-9-]/g, ""), 10);
    return isNaN(num) ? 0 : num;
  };

  const parseMs = (d?: string, fallback = 0) => {
    if (!d) return fallback;
    const num = parseInt(d.replace(/[^0-9]/g, ""), 10);
    return isNaN(num) ? fallback : num;
  };

  useEffect(() => {
    if (editingPersona) {
      setId(editingPersona.id);
      setName(editingPersona.name);
      setDescription(editingPersona.description || "");
      setVoice(editingPersona.voice);
      setFormat(editingPersona.format || "mp3");
      setSpeed(editingPersona.speed || 1.0);
      setPitchHz(parseHz(editingPersona.pitch));
      setBreakCommaMs(parseMs(editingPersona.break_comma, 150));
      setBreakPeriodMs(parseMs(editingPersona.break_period, 350));
      setTelephony(editingPersona.telephony || false);
      setSanitize(!editingPersona.remove_filter);
      setEmotion(editingPersona.emotion || "");
      setEmotionScale(editingPersona.emotion_scale ? parseFloat(editingPersona.emotion_scale) : 1.0);
      setIsIdCustomized(true);
    } else {
      setId("");
      setName("");
      setDescription("");
      setVoice(voices[0]?.id || "pt-BR-ThalitaMultilingualNeural");
      setFormat("mp3");
      setSpeed(1.0);
      setPitchHz(0);
      setBreakCommaMs(150);
      setBreakPeriodMs(350);
      setTelephony(false);
      setSanitize(true);
      setEmotion("");
      setEmotionScale(1.0);
      setIsIdCustomized(false);
    }
  }, [editingPersona, open, voices]);

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setName(val);
    if (!editingPersona && !isIdCustomized) {
      setId(slugify(val));
    }
  };

  const handleIdChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setIsIdCustomized(true);
    setId(slugify(e.target.value));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const finalId = slugify(id || name);
    if (!name.trim() || (!editingPersona && !finalId)) return;

    const formattedPitch = pitchHz >= 0 ? `+${pitchHz}Hz` : `${pitchHz}Hz`;
    const formattedBreakComma = breakCommaMs > 0 ? `${breakCommaMs}ms` : "";
    const formattedBreakPeriod = breakPeriodMs > 0 ? `${breakPeriodMs}ms` : "";

    const selectedVoiceObj = voices.find(
      (v) => v.id.toLowerCase() === voice.toLowerCase() || (v.aliases && v.aliases.includes(voice.toLowerCase()))
    );

    const isCapCutVoice =
      selectedVoiceObj?.provider === "capcut" ||
      voice.toLowerCase().startsWith("dit_") ||
      voice.toLowerCase().startsWith("icl_") ||
      voice.toLowerCase().startsWith("bv") ||
      voice.toLowerCase() === "wenrou" ||
      voice.toLowerCase() === "empresario" ||
      voice.toLowerCase() === "luiz" ||
      voice.toLowerCase() === "youya" ||
      voice.toLowerCase() === "emotional" ||
      voice.toLowerCase() === "dramaaa";

    setIsSubmitting(true);
    try {
      await onSave({
        id: finalId,
        name: name.trim(),
        description: description.trim(),
        voice,
        provider: selectedVoiceObj?.provider || (isCapCutVoice ? "capcut" : "edge"),
        format,
        speed,
        pitch: formattedPitch,
        break_comma: formattedBreakComma,
        break_period: formattedBreakPeriod,
        telephony,
        remove_filter: !sanitize,
        emotion: isCapCutVoice && emotion ? emotion : undefined,
        emotion_scale: isCapCutVoice && emotion ? emotionScale.toFixed(1) : undefined,
      });
      onOpenChange(false);
    } finally {
      setIsSubmitting(false);
    }
  };

  const selectedVoiceObj = voices.find(
    (v) => v.id.toLowerCase() === voice.toLowerCase() || (v.aliases && v.aliases.includes(voice.toLowerCase()))
  );

  const isCapCutVoice =
    selectedVoiceObj?.provider === "capcut" ||
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
    <Dialog open={open} onOpenChange={onOpenChange}>
      <form onSubmit={handleSubmit}>
        <DialogHeader onClose={() => onOpenChange(false)}>
          <DialogTitle>
            {editingPersona ? t("modalEditTitle") : t("modalNewTitle")}
          </DialogTitle>
          <DialogDescription>
            {t("modalDescription")}
          </DialogDescription>
        </DialogHeader>

        <DialogContent className="space-y-4 max-h-[75vh] overflow-y-auto pr-1">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              {t("displayNameLabel")}
            </label>
            <Input
              value={name}
              onChange={handleNameChange}
              placeholder={t("displayNamePlaceholder")}
              required
              className="h-9 text-xs"
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex justify-between items-center">
              <label className="text-xs font-medium text-muted-foreground">
                {t("uniqueIdLabel")}
              </label>
              <span className="text-[10px] text-muted-foreground font-mono">
                {t("uniqueIdHint")}
              </span>
            </div>
            <Input
              value={id}
              onChange={handleIdChange}
              placeholder={t("uniqueIdPlaceholder")}
              disabled={!!editingPersona}
              className="font-mono text-xs h-9"
              required
            />
            <span className="text-[10px] text-muted-foreground font-mono">
              {t("endpointPreview")}: /v1/persona/{id || "slug"}/speech
            </span>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              {t("descriptionLabel")}
            </label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t("descriptionPlaceholder")}
              rows={2}
              className="text-xs resize-none"
            />
          </div>

          {/* Voice Select */}
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
                  <span>Sentimentos & Expressividade (CapCut)</span>
                </span>
                <span className="text-[9px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-300 border border-purple-500/30 font-medium">
                  Nativo ByteDance
                </span>
              </div>

              {/* Card de Personalidade Nativa da Voz */}
              {selectedVoiceObj?.personality && (
                <div className="p-2 rounded-lg bg-purple-950/50 border border-purple-500/20 text-[11px] text-purple-200/90 leading-relaxed">
                  <span className="font-semibold text-purple-300 block mb-0.5">
                    ✨ Personalidade Nativa:
                  </span>
                  {selectedVoiceObj.personality}
                </div>
              )}

              {/* Seletor de Emoção / Sentimento em Pills */}
              <div className="space-y-1.5">
                <div className="flex justify-between text-[11px] text-muted-foreground">
                  <span>Sentimento Padrão da Persona:</span>
                  <span className="font-mono text-purple-300 font-medium capitalize">
                    {emotion || "Natural"}
                  </span>
                </div>
                <div className="grid grid-cols-4 gap-1">
                  {[
                    { id: "", label: "Natural", icon: "😐" },
                    { id: "calm", label: "Calmo", icon: "😌" },
                    { id: "happy", label: "Feliz", icon: "😊" },
                    { id: "angry", label: "Firme", icon: "😠" },
                    { id: "sad", label: "Triste", icon: "😢" },
                    { id: "surprise", label: "Surpreso", icon: "😲" },
                    { id: "fear", label: "Tenso", icon: "😱" },
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
                    <span>Intensidade do Sentimento:</span>
                    <span className="font-mono text-purple-300 font-semibold">
                      {emotionScale.toFixed(1)}x
                      {emotionScale <= 0.6 ? " (Sutil)" : emotionScale <= 1.2 ? " (Equilibrado)" : " (Intenso)"}
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

          {/* Format & Speed */}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground">
                {t("outputFormatLabel")}
              </label>
              <select
                value={format}
                onChange={(e) => setFormat(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1.5 text-xs text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring transition-colors"
              >
                <option value="mp3">MP3 (Universal)</option>
                <option value="opus">Opus (WhatsApp / Telegram)</option>
                <option value="wav">WAV (Sem perdas)</option>
                <option value="pcm">PCM</option>
                <option value="flac">FLAC</option>
                <option value="aac">AAC</option>
              </select>
            </div>

            <div className="space-y-1.5">
              <div className="flex justify-between items-center">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("speedLabel")}
                </label>
                <span className="text-xs font-mono text-muted-foreground">
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
                className="w-full mt-2.5 accent-primary h-1.5 bg-secondary rounded-lg cursor-pointer"
              />
            </div>
          </div>

          {/* Pitch (Tom da Voz) */}
          <div className="p-3 bg-secondary/30 border border-border/80 rounded-lg space-y-2">
            <div className="flex justify-between items-center">
              <div className="flex items-center gap-1.5">
                <IonIcon name="musical-note-outline" className="text-primary text-xs" />
                <label className="text-xs font-medium text-foreground">
                  {t("pitchLabel")}
                </label>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs font-mono font-semibold text-primary">
                  {pitchHz >= 0 ? `+${pitchHz}Hz` : `${pitchHz}Hz`}
                </span>
                {pitchHz !== 0 && (
                  <button
                    type="button"
                    onClick={() => setPitchHz(0)}
                    className="text-[10px] text-muted-foreground hover:text-foreground font-mono underline"
                  >
                    {t("resetDefault")}
                  </button>
                )}
              </div>
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
            <p className="text-[10px] text-muted-foreground">
              {t("pitchDesc")}
            </p>
          </div>

          {/* Pausas e Expressividade SSML */}
          <div className="p-3 bg-secondary/30 border border-border/80 rounded-lg space-y-3">
            <div className="flex items-center gap-1.5">
              <IonIcon name="timer-outline" className="text-primary text-xs" />
              <span className="text-xs font-medium text-foreground">
                {t("ssmlBreakSectionTitle")}
              </span>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <div className="flex justify-between items-center">
                  <label className="text-[11px] font-medium text-muted-foreground">
                    {t("breakCommaLabel")}
                  </label>
                  <span className="text-[11px] font-mono text-foreground font-semibold">
                    {breakCommaMs}ms
                  </span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="500"
                  step="25"
                  value={breakCommaMs}
                  onChange={(e) => setBreakCommaMs(parseInt(e.target.value, 10))}
                  className="w-full accent-primary h-1.5 bg-secondary rounded-lg cursor-pointer"
                />
              </div>

              <div className="space-y-1.5">
                <div className="flex justify-between items-center">
                  <label className="text-[11px] font-medium text-muted-foreground">
                    {t("breakPeriodLabel")}
                  </label>
                  <span className="text-[11px] font-mono text-foreground font-semibold">
                    {breakPeriodMs}ms
                  </span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="1000"
                  step="50"
                  value={breakPeriodMs}
                  onChange={(e) => setBreakPeriodMs(parseInt(e.target.value, 10))}
                  className="w-full accent-primary h-1.5 bg-secondary rounded-lg cursor-pointer"
                />
              </div>
            </div>
            <p className="text-[10px] text-muted-foreground">
              {t("ssmlBreakHint")}
            </p>
          </div>

          {/* Telephony DSP Filter */}
          <FieldLabel htmlFor="switch-telephony">
            <Field orientation="horizontal">
              <FieldContent>
                <FieldTitle>Modo Linha Telefônica (DSP 3.4kHz)</FieldTitle>
                <FieldDescription>
                  Aplica corte passa-faixa (300Hz-3.4kHz) e equalização de headset para simular atendimento telefônico real
                </FieldDescription>
              </FieldContent>
              <Switch
                id="switch-telephony"
                checked={telephony}
                onCheckedChange={setTelephony}
              />
            </Field>
          </FieldLabel>


          {/* Text Sanitizer Filter using Shadcn Field & Switch */}
          <FieldLabel htmlFor="switch-sanitize">
            <Field orientation="horizontal">
              <FieldContent>
                <FieldTitle>{t("smartSanitizerTitle")}</FieldTitle>
                <FieldDescription>
                  {t("smartSanitizerDesc")}
                </FieldDescription>
              </FieldContent>
              <Switch
                id="switch-sanitize"
                checked={sanitize}
                onCheckedChange={setSanitize}
              />
            </Field>
          </FieldLabel>
        </DialogContent>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            className="h-8 text-xs"
          >
            {t("cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isSubmitting || !name.trim()}
            className="h-8 text-xs font-medium"
          >
            {isSubmitting ? (
              <>
                <IonIcon name="sync-outline" className="text-xs animate-spin mr-1.5" />
                <span>{t("saving")}</span>
              </>
            ) : (
              <>
                <IonIcon name="checkmark-outline" className="text-xs mr-1.5" />
                <span>{editingPersona ? t("saveChangesButton") : t("createPersonaSubmit")}</span>
              </>
            )}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}

