/** Summary item from list endpoints (no content). */
export interface PostSummary {
  id: string;
  user_id: string;
  title: string;
  slug: string;
  published: boolean;
  created_at: string;
  updated_at: string;
}

/** Full post from create/get endpoints. */
export interface Post {
  id: string;
  user_id: string;
  title: string;
  content: string;
  slug: string;
  published: boolean;
  created_at: string;
  updated_at: string;
}

/** Response from list all / list user posts. */
export interface ListPostsResponse {
  posts: PostSummary[];
  limit: number;
  offset: number;
  total: number;
}
