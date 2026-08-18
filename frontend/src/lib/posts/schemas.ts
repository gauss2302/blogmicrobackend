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

/**
 * Partial update. Mirrors the gateway's `UpdatePostRequest`, where every field
 * is a pointer with `omitempty`: an omitted key means "leave unchanged", which
 * is why nothing here has a default. Sending `title: ""` would be a real edit
 * to an empty title, so the same min/max bounds as create still apply.
 */
export const updatePostSchema = z
  .object({
    title: z
      .string()
      .trim()
      .min(1, "Title is required.")
      .max(200, "Title must be at most 200 characters.")
      .optional(),
    content: z
      .string()
      .trim()
      .min(1, "Content is required.")
      .max(50_000, "Content must be at most 50,000 characters.")
      .optional(),
    slug: z
      .string()
      .trim()
      .min(3, "Slug must be at least 3 characters.")
      .max(100, "Slug must be at most 100 characters.")
      .optional(),
    published: z.boolean().optional(),
  })
  .refine((value) => Object.values(value).some((v) => v !== undefined), {
    message: "At least one field must be provided.",
  });

export type UpdatePostInput = z.infer<typeof updatePostSchema>;
