import type { VoteOrNo } from './types';

/** The vote following v when a participant clicks a cell: no → yes → maybe → no. */
export function nextVote(v: VoteOrNo): VoteOrNo {
  return v === 'no' ? 'yes' : v === 'yes' ? 'maybe' : 'no';
}
