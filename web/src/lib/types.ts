export interface User {
  id: string;
  email: string;
  name: string;
  picture: string;
}

export type Mode = 'days' | 'hours';
export type Vote = 'yes' | 'maybe';
export type VoteOrNo = Vote | 'no';

export interface Slot {
  id: string;
  start: string;
  end: string;
}

export interface SlotInput {
  id?: string;
  start: string;
  end: string;
}

export interface Response {
  user: User;
  votes: Record<string, Vote>;
  updatedAt: string;
}

export interface EventDetail {
  id: string;
  title: string;
  description: string;
  location: string;
  mode: Mode;
  owner: User;
  slots: Slot[];
  closed: boolean;
  finalSlotId?: string;
  createdAt: string;
  updatedAt: string;
  responses: Response[];
}

export interface EventSummary {
  id: string;
  title: string;
  location: string;
  mode: Mode;
  owner: User;
  role: 'owner' | 'participant';
  closed: boolean;
  slotCount: number;
  responseCount: number;
  firstStart?: string;
  lastEnd?: string;
  createdAt: string;
  updatedAt: string;
}

export interface EventInput {
  title: string;
  description: string;
  location: string;
  mode: Mode;
  slots: SlotInput[];
}
