import { z } from "zod";

/**
 * Partial profile update. Mirrors the gateway's `UpdateUserInput`, where every
 * field is an optional pointer — an omitted key means "leave unchanged".
 *
 * `id` and `actor_id` are deliberately absent: the gateway fills them from the
 * URL path and the access token respectively (`json:"-"`), and user-service
 * enforces `actor_id == id`. Accepting either from the client would be an
 * authorization hole, so they must never appear in this schema.
 */
export const updateUserSchema = z
  .object({
    name: z
      .string()
      .trim()
      .min(1, "Name cannot be empty.")
      .max(100, "Name must be at most 100 characters.")
      .optional(),
    picture: z
      .string()
      .trim()
      .url("Picture must be a valid URL.")
      .max(2048, "Picture URL is too long.")
      .or(z.literal(""))
      .optional(),
    bio: z
      .string()
      .trim()
      .max(500, "Bio must be at most 500 characters.")
      .optional(),
    location: z
      .string()
      .trim()
      .max(100, "Location must be at most 100 characters.")
      .optional(),
    website: z
      .string()
      .trim()
      .url("Website must be a valid URL.")
      .max(2048, "Website URL is too long.")
      .or(z.literal(""))
      .optional(),
  })
  .refine((value) => Object.values(value).some((v) => v !== undefined), {
    message: "At least one field must be provided.",
  });

export type UpdateUserInput = z.infer<typeof updateUserSchema>;
