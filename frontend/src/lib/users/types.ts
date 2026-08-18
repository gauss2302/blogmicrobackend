/**
 * Authenticated view of a user. Mirrors the gateway's `UserResponse` — includes
 * email and activity flags, so it is only ever served on protected routes.
 */
export interface User {
  id: string;
  email: string;
  name: string;
  picture?: string;
  bio?: string;
  location?: string;
  website?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Public/discovery view. Mirrors the gateway's `UserProfileResponse`, which
 * deliberately omits email and other PII so it can be served unauthenticated.
 * Do not widen this to include email — the backend does not send it, and the
 * omission is intentional.
 */
export interface UserProfile {
  id: string;
  name: string;
  picture?: string;
  bio?: string;
  location?: string;
  website?: string;
}

/**
 * Cursor-paginated follower/following list. `next_cursor` is absent on the
 * final page (the backend tags it `omitempty`).
 */
export interface ListFollowResponse {
  users: UserProfile[];
  next_cursor?: string;
}

export interface UserStats {
  total_active_users: number;
}
