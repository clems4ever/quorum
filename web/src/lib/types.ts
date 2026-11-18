export interface User {
  id: string;
  email: string;
  name: string;
  picture: string;
}

export type Mode = 'days' | 'hours';

/** The point of an image kept visible when it is cropped, as fractions (0.5, 0.5 is the center). */
export interface Focus {
  x: number;
  y: number;
}

export interface Cover {
  url: string;
  focus: Focus;
}
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
  cover?: Cover;
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
  cover?: Cover;
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
