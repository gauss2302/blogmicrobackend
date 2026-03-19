import { z } from "zod";

export const createPostSchema = z.object({
  title: z
    .string()
    .trim()
    .min(1, "Title is required.")
    .max(200, "Title must be at most 200 characters."),
  content: z
    .string()
    .trim()
    .min(1, "Content is required.")
    .max(50_000, "Content must be at most 50,000 characters."),
  slug: z
    .string()
    .trim()
    .min(3, "Slug must be at least 3 characters.")
    .max(100, "Slug must be at most 100 characters.")
    .optional()
    .or(z.literal("")),
  published: z.boolean().optional().default(true),
});

export type CreatePostInput = z.infer<typeof createPostSchema>;
