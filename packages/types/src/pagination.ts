/** Opaque, forward-only pagination cursor. */
export type PaginationCursor = string;

export interface PaginationParams {
  readonly limit: number;
  readonly cursor?: PaginationCursor;
}

export interface PaginatedResponse<T> {
  readonly items: readonly T[];
  readonly nextCursor: PaginationCursor | null;
}
