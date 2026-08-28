/**
 * S_solar's exact orientation base-score table (PRD §5.8,
 * services/api/internal/scoring/factor_solar.go CalculateSolarScore) --
 * kept in sync by hand since this is presentation-only (what the sidebar
 * badge shows), not the scoring computation itself, which stays
 * server-side.
 */
const SOLAR_BASE_SCORES: Record<string, number> = {
  S: 100,
  SE: 92.5,
  SW: 87.5,
  E: 85,
  W: 75,
  NE: 62.5,
  NW: 57.5,
  N: 40,
};

/**
 * Maps an 8-point compass facing to its S_solar orientation base score.
 * An unrecognized value falls back to 100 -- factor_solar.go's own
 * baseScore starts at 100.0 before its switch statement narrows it for a
 * recognized facing, so an unrecognized one keeps that same default.
 */
export function solarBaseScore(facing: string): number {
  return SOLAR_BASE_SCORES[facing.trim().toUpperCase()] ?? 100;
}

export type ScoreTier = "emerald" | "lime" | "amber" | "rose";

/**
 * Maps an S_solar base score to its DESIGN.md score tier (Optimal/Emerald
 * 80-100, Good/Lime 65-79, Moderate/Amber 45-64, Alert/Rose 0-44) --
 * reused here deliberately: unlike the POI/Divvy category markers, this
 * badge genuinely represents a score, so the score palette is the
 * *correct* semantic fit, not a borrowed one. The tier name is what the
 * sidebar's `<Badge variant={`score-${tier}`}>` needs.
 */
export function solarScoreTier(score: number): ScoreTier {
  if (score >= 80) return "emerald";
  if (score >= 65) return "lime";
  if (score >= 45) return "amber";
  return "rose";
}
