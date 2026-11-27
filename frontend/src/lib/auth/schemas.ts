import { z } from "zod";

import { utf8Bytes } from "@/lib/utils";

// bcrypt (user-service) rejects passwords over 72 bytes.
const password = z
  .string()
  .min(8, "Password must be at least 8 characters.")
  .refine((v) => utf8Bytes(v) <= 72, "Password must be at most 72 bytes.");

export const loginSchema = z.object({
  email: z.email().trim(),
  password,
});

export const registerSchema = z.object({
  email: z.email().trim(),
  password,
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .refine((v) => utf8Bytes(v) <= 100, "Name must be at most 100 bytes."),
});

export const exchangeAuthCodeSchema = z.object({
  authCode: z.string().trim().min(1, "Auth code is required."),
  codeVerifier: z.string().trim().optional(),
});

export type LoginInput = z.infer<typeof loginSchema>;
export type RegisterInput = z.infer<typeof registerSchema>;
export type ExchangeAuthCodeInput = z.infer<typeof exchangeAuthCodeSchema>;
