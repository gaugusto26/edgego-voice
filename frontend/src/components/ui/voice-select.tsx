"use client";

import React, { useState, useRef, useEffect } from "react";
import { VoiceOption } from "@/types";
import { CountryFlag } from "@/components/ui/country-flag";
import { IonIcon } from "@/components/ui/ion-icon";
import { useLanguage } from "@/i18n/LanguageContext";

interface VoiceSelectProps {
  voices: VoiceOption[];
  value: string;
  onChange: (value: string) => void;
  className?: string;
}

export function VoiceSelect({
  voices,
  value,
  onChange,
  className = "",
}: VoiceSelectProps) {
  const { t } = useLanguage();
  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [filterProvider, setFilterProvider] = useState<"all" | "capcut" | "edge">("all");
  const containerRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  const selectedVoice =
    voices.find(
      (v) =>
        v.id.toLowerCase() === value?.toLowerCase() ||
        (v.aliases && v.aliases.some((a) => a.toLowerCase() === value?.toLowerCase()))
    ) || voices[0];

  const isCapCut =
    selectedVoice?.provider === "capcut" ||
    (selectedVoice?.id &&
      (selectedVoice.id.toLowerCase().startsWith("dit_") ||
        selectedVoice.id.toLowerCase().startsWith("icl_") ||
        selectedVoice.id.toLowerCase().startsWith("bv") ||
        selectedVoice.id.toLowerCase() === "wenrou" ||
        selectedVoice.id.toLowerCase() === "empresario" ||
        selectedVoice.id.toLowerCase() === "luiz" ||
        selectedVoice.id.toLowerCase() === "youya" ||
        selectedVoice.id.toLowerCase() === "emotional" ||
        selectedVoice.id.toLowerCase() === "dramaaa"));

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
        setSearch("");
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  useEffect(() => {
    if (isOpen) {
      setTimeout(() => searchRef.current?.focus(), 50);
    }
  }, [isOpen]);

  const capcutCount = voices.filter(
    (v) =>
      v.provider === "capcut" ||
      v.id.toLowerCase().startsWith("dit_") ||
      v.id.toLowerCase().startsWith("icl_") ||
      v.id.toLowerCase().startsWith("bv") ||
      v.id.toLowerCase() === "wenrou" ||
      v.id.toLowerCase() === "empresario" ||
      v.id.toLowerCase() === "luiz" ||
      v.id.toLowerCase() === "youya" ||
      v.id.toLowerCase() === "emotional" ||
      v.id.toLowerCase() === "dramaaa"
  ).length;
  const edgeCount = voices.length - capcutCount;

  const filteredVoices = voices.filter((v) => {
    const isVUnderCapCut =
      v.provider === "capcut" ||
      v.id.toLowerCase().startsWith("dit_") ||
      v.id.toLowerCase().startsWith("icl_") ||
      v.id.toLowerCase().startsWith("bv") ||
      v.id.toLowerCase() === "wenrou" ||
      v.id.toLowerCase() === "empresario" ||
      v.id.toLowerCase() === "luiz" ||
      v.id.toLowerCase() === "youya" ||
      v.id.toLowerCase() === "emotional" ||
      v.id.toLowerCase() === "dramaaa";

    const vProvider = isVUnderCapCut ? "capcut" : "edge";
    if (filterProvider !== "all" && vProvider !== filterProvider) {
      return false;
    }
    const q = search.toLowerCase().trim();
    if (!q) return true;

    return (
      v.id.toLowerCase().includes(q) ||
      v.name.toLowerCase().includes(q) ||
      vProvider.includes(q) ||
      v.language.toLowerCase().includes(q) ||
      (v.country_code && v.country_code.toLowerCase().includes(q)) ||
      (v.personality && v.personality.toLowerCase().includes(q)) ||
      (v.description && v.description.toLowerCase().includes(q)) ||
      (v.aliases && v.aliases.some((a) => a.toLowerCase().includes(q)))
    );
  });

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      {/* Trigger */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`flex h-auto w-full items-center justify-between rounded-xl border px-3.5 py-2.5 text-sm text-foreground transition-all duration-200 shadow-sm ${
          isOpen
            ? "border-primary bg-card ring-1 ring-primary/30"
            : "border-border bg-card hover:border-zinc-600"
        }`}
      >
        <div className="flex items-center gap-2.5 truncate">
          {selectedVoice && (
            <CountryFlag
              countryCode={selectedVoice.country_code || "BR"}
              className="w-5 h-3.5 rounded-[3px] shadow-sm shrink-0"
            />
          )}
          <div className="text-left truncate">
            <div className="flex items-center gap-1.5 truncate">
              <span className="font-bold text-foreground truncate">
                {selectedVoice?.name}
              </span>
              {isCapCut ? (
                <span className="text-[9px] font-medium px-1.5 py-0.2 rounded bg-purple-500/15 text-purple-300 border border-purple-500/30 shrink-0">
                  🎭 CapCut Expressiva
                </span>
              ) : (
                <span className="text-[9px] font-medium px-1.5 py-0.2 rounded bg-sky-500/10 text-sky-300 border border-sky-500/20 shrink-0">
                  ⚡ Edge
                </span>
              )}
            </div>
            <div className="text-xs text-muted-foreground truncate">
              {selectedVoice?.language} • {selectedVoice?.gender}
              {selectedVoice?.personality && (
                <span className="text-purple-300/80 ml-1.5 truncate">
                  — {selectedVoice.personality}
                </span>
              )}
            </div>
          </div>
        </div>
        <IonIcon
          name="chevron-down-outline"
          className={`text-sm text-muted-foreground transition-transform duration-200 ml-2 shrink-0 ${
            isOpen ? "rotate-180 text-primary" : ""
          }`}
        />
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div className="absolute left-0 right-0 z-50 mt-2 overflow-hidden rounded-xl border border-border bg-card shadow-2xl flex flex-col animate-in fade-in-0 slide-in-from-top-2 duration-150">
          {/* Search & Provider Filter Tabs */}
          <div className="p-2.5 border-b border-border/60 space-y-2 bg-card">
            <div className="relative flex items-center">
              <IonIcon
                name="search-outline"
                className="absolute left-3 text-xs text-muted-foreground pointer-events-none"
              />
              <input
                ref={searchRef}
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t("searchVoicePlaceholder")}
                className="w-full bg-background/60 rounded-lg border border-border py-1.5 pl-8 pr-3 text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary transition-colors"
              />
            </div>

            {/* Quick Provider Filter Tabs */}
            <div className="flex items-center gap-1.5 pt-0.5">
              <button
                type="button"
                onClick={() => setFilterProvider("all")}
                className={`text-[10px] font-medium px-2 py-0.5 rounded-md transition-colors ${
                  filterProvider === "all"
                    ? "bg-secondary text-foreground font-semibold"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {t("filterAll")} ({voices.length})
              </button>
              <button
                type="button"
                onClick={() => setFilterProvider("capcut")}
                className={`text-[10px] font-medium px-2 py-0.5 rounded-md transition-colors flex items-center gap-1 ${
                  filterProvider === "capcut"
                    ? "bg-purple-500/20 text-purple-300 border border-purple-500/40 font-semibold"
                    : "text-purple-400/70 hover:text-purple-300"
                }`}
              >
                <span>🎭 CapCut ({capcutCount})</span>
              </button>
              <button
                type="button"
                onClick={() => setFilterProvider("edge")}
                className={`text-[10px] font-medium px-2 py-0.5 rounded-md transition-colors flex items-center gap-1 ${
                  filterProvider === "edge"
                    ? "bg-sky-500/20 text-sky-300 border border-sky-500/40 font-semibold"
                    : "text-sky-400/70 hover:text-sky-300"
                }`}
              >
                <span>⚡ Edge ({edgeCount})</span>
              </button>
            </div>
          </div>

          {/* List */}
          <div className="overflow-y-auto p-1.5 space-y-1 max-h-72 scrollbar-thin">
            {filteredVoices.map((v) => {
              const isSelected = v.id === value;
              const vIsCapCut = v.provider === "capcut";

              return (
                <button
                  key={v.id}
                  type="button"
                  onClick={() => {
                    onChange(v.id);
                    setIsOpen(false);
                    setSearch("");
                  }}
                  className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm transition-all ${
                    isSelected
                      ? vIsCapCut
                        ? "bg-purple-500/15 border border-purple-500/40 text-purple-200"
                        : "bg-primary/10 border border-primary/30 text-primary"
                      : "text-foreground hover:bg-white/5 border border-transparent"
                  }`}
                >
                  <div className="flex items-center gap-2.5 truncate min-w-0 flex-1">
                    <CountryFlag
                      countryCode={v.country_code || "BR"}
                      className="w-5 h-3.5 rounded-[3px] shadow-sm shrink-0"
                    />
                    <div className="truncate min-w-0 flex-1">
                      <div className="flex items-center gap-1.5 truncate">
                        <span className={`font-semibold text-xs ${isSelected ? (vIsCapCut ? "text-purple-200" : "text-primary") : "text-foreground"}`}>
                          {v.name}
                        </span>
                        {vIsCapCut ? (
                          <span className="text-[9px] font-medium px-1.5 py-0.2 rounded bg-purple-500/20 text-purple-300 border border-purple-500/30">
                            CapCut
                          </span>
                        ) : (
                          <span className="text-[9px] font-medium px-1.5 py-0.2 rounded bg-zinc-800 text-zinc-400 border border-zinc-700/50">
                            Edge
                          </span>
                        )}
                        <span className="text-[10px] text-muted-foreground font-normal">
                          • {v.gender}
                        </span>
                      </div>
                      <div className="text-[10px] text-muted-foreground truncate">
                        {v.language}
                        {v.personality && (
                          <span className="text-purple-300/90 font-medium ml-1.5">
                            — {v.personality}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  {isSelected && (
                    <IonIcon
                      name="checkmark-outline"
                      className={`text-base ml-2 shrink-0 ${vIsCapCut ? "text-purple-300" : "text-primary"}`}
                    />
                  )}
                </button>
              );
            })}

            {filteredVoices.length === 0 && (
              <div className="p-6 text-center text-xs text-muted-foreground">
                <IonIcon name="search-outline" className="text-2xl mb-2 block mx-auto opacity-30" />
                {t("noVoicesFound")}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
