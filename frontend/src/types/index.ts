export interface Persona {
  id: string;
  name: string;
  description: string;
  voice: string;
  provider?: "capcut" | "edge" | string;
  emotion?: string;
  emotion_scale?: string;
  style?: string;
  role?: string;
  format: string;
  speed: number;
  pitch: string;
  break_comma?: string;
  break_period?: string;
  telephony?: boolean;
  auto_breath?: boolean;
  remove_filter: boolean;
  api_key?: string;
  created_at?: string;
  updated_at?: string;
}

export interface VoiceOption {
  id: string;
  name: string;
  language: string;
  gender: string;
  country_code?: string;
  flag?: string;
  locale?: string;
  description: string;
  provider?: "capcut" | "edge" | string;
  personality?: string;
  emotions?: string[];
  aliases?: string[];
}

export type TabType = "personas" | "playground" | "apidocs";

export interface SubtitleWord {
  word: string;
  start_ms: number;
  end_ms: number;
}

export interface SubtitleCue {
  index: number;
  text: string;
  start_ms: number;
  end_ms: number;
  words?: SubtitleWord[];
}

export interface SubtitleResult {
  text: string;
  duration_ms: number;
  srt: string;
  vtt: string;
  sentences: SubtitleCue[];
  words?: SubtitleWord[];
}

