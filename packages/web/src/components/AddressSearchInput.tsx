import { useId, useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import "lineicons/dist/lineicons.css";
import { apiClient } from "../lib/api/client";
import { cn } from "../lib/utils";
import type { components } from "../lib/api/schema";

type GeocodeCandidate = components["schemas"]["GeocodeCandidate"];
type ScoreResponse = components["schemas"]["ScoreResponse"];

export type ResolvedAddress = {
  formattedAddress: string;
  lat: number;
  lon: number;
  score: ScoreResponse;
};

export type AddressSearchInputProps = {
  /** Fires once geocoding *and* scoring have both succeeded for a chosen address. */
  onResolved: (resolved: ResolvedAddress) => void;
  /** Fires when the user starts a new search, so the caller can clear any previously resolved address/layers immediately. */
  onReset: () => void;
};

type FlowState =
  | { status: "idle" }
  | { status: "geocoding" }
  | { status: "not-found" }
  | { status: "ambiguous"; candidates: GeocodeCandidate[] }
  | { status: "scoring"; formattedAddress: string }
  | { status: "out-of-coverage"; formattedAddress: string }
  | { status: "error"; message: string };

/**
 * Sidebar address entry point (docs/IMPLEMENTATION_PHASES.md Phase 5
 * deliverable 1): resolves free-text address input through
 * `GET /api/v1/geocode`, disambiguating multiple candidates with a picker,
 * then scores the resolved coordinate via `POST /api/v1/score`. The first
 * real user-supplied-address entry point anywhere in the app -- every
 * earlier layer/endpoint queried a fixed demo coordinate instead.
 */
export function AddressSearchInput({ onResolved, onReset }: AddressSearchInputProps) {
  const [query, setQuery] = useState("");
  const [flow, setFlow] = useState<FlowState>({ status: "idle" });
  const inputId = useId();

  const geocodeMutation = useMutation({
    mutationFn: async (q: string) => {
      const { data, error } = await apiClient.GET("/api/v1/geocode", { params: { query: { q } } });
      if (error) throw new Error("geocode failed");
      return data;
    },
  });

  const scoreMutation = useMutation({
    mutationFn: async (candidate: GeocodeCandidate) => {
      // Scores by the exact resolved `origin` coordinates, not by
      // re-sending `address` text -- sending `address` makes the server
      // re-geocode it (resolveCoordinates, handlers.go) purely to recover
      // a coordinate we already have, and NominatimClient rate-limits to 1
      // req/sec, so that redundant geocode blocked every single search for
      // close to a full second (confirmed live, PR #62 review). `origin`
      // alone would silently default S_solar's street-facing inference to
      // South (100/100), since InferStreetFacingOrientation needs address
      // text -- addressHint supplies that same text purely for orientation
      // inference, without ever triggering a geocode (docs/openapi.yaml
      // ScoreRequest.addressHint).
      const { data, error } = await apiClient.POST("/api/v1/score", {
        body: { origin: candidate.coordinates, addressHint: candidate.formattedAddress },
      });
      if (error) throw error;
      return data;
    },
  });

  async function resolveCandidate(candidate: GeocodeCandidate) {
    setFlow({ status: "scoring", formattedAddress: candidate.formattedAddress });
    try {
      const score = await scoreMutation.mutateAsync(candidate);
      setFlow({ status: "idle" });
      onResolved({
        formattedAddress: candidate.formattedAddress,
        lat: candidate.coordinates.lat,
        lon: candidate.coordinates.lon,
        score,
      });
    } catch (err) {
      const code = (err as { code?: string } | undefined)?.code;
      if (code === "OUT_OF_COVERAGE") {
        setFlow({ status: "out-of-coverage", formattedAddress: candidate.formattedAddress });
      } else {
        setFlow({ status: "error", message: "Couldn't score that address. Try again." });
      }
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = query.trim();
    if (!trimmed) return;

    onReset();
    setFlow({ status: "geocoding" });
    try {
      const result = await geocodeMutation.mutateAsync(trimmed);
      const candidates = result?.candidates ?? [];
      if (candidates.length === 0) {
        setFlow({ status: "not-found" });
      } else if (candidates.length === 1) {
        await resolveCandidate(candidates[0]);
      } else {
        setFlow({ status: "ambiguous", candidates });
      }
    } catch {
      setFlow({ status: "error", message: "Couldn't reach the geocoder. Check your connection and try again." });
    }
  }

  const isBusy = flow.status === "geocoding" || flow.status === "scoring";

  return (
    <div className="space-y-2">
      <form onSubmit={handleSubmit} className="flex items-center gap-2">
        <label htmlFor={inputId} className="sr-only">
          Address
        </label>
        <div className="relative flex-1">
          <i className="lni lni-search-1 pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-xs text-zinc-500" />
          <input
            id={inputId}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search a Chicago address"
            disabled={isBusy}
            className="w-full rounded-lg border border-zinc-800 bg-zinc-950 py-2 pr-3 pl-8 font-mono text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-zinc-600 focus:outline-none disabled:opacity-50"
          />
        </div>
        <button
          type="submit"
          disabled={isBusy || !query.trim()}
          className="rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2 text-xs font-mono font-medium tracking-wide text-zinc-100 uppercase transition-colors hover:bg-zinc-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isBusy ? <i className="lni lni-spinner-3 animate-spin" /> : "Search"}
        </button>
      </form>

      {flow.status === "not-found" && (
        <AddressSearchNotice tone="rose" text={`No address matched "${query.trim()}".`} />
      )}

      {flow.status === "error" && <AddressSearchNotice tone="rose" text={flow.message} />}

      {flow.status === "out-of-coverage" && (
        <AddressSearchNotice
          tone="amber"
          text={`${flow.formattedAddress} falls outside the City of Chicago boundary, which BlockFactor doesn't cover yet.`}
        />
      )}

      {flow.status === "ambiguous" && (
        <div className="space-y-1 rounded-sm border border-zinc-800 bg-zinc-900/80 p-1.5">
          <div className="px-1.5 py-1 text-xs font-mono text-zinc-500 uppercase tracking-wider">
            {flow.candidates.length} matches -- pick one
          </div>
          <ul>
            {flow.candidates.map((candidate) => (
              <li key={`${candidate.rank}-${candidate.formattedAddress}`}>
                <button
                  type="button"
                  onClick={() => resolveCandidate(candidate)}
                  className="w-full rounded-sm px-2 py-1.5 text-left text-sm text-zinc-200 transition-colors hover:bg-zinc-800"
                >
                  {candidate.formattedAddress}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function AddressSearchNotice({ tone, text }: { tone: "rose" | "amber"; text: string }) {
  return (
    <div
      className={cn(
        "flex items-start gap-1.5 rounded-sm border px-2 py-1.5 text-xs font-mono",
        tone === "rose" ? "border-rose-900/60 bg-rose-950/40 text-rose-400" : "border-amber-900/60 bg-amber-950/40 text-amber-400"
      )}
    >
      <i className="lni lni-xmark-circle mt-px shrink-0" />
      <span>{text}</span>
    </div>
  );
}
