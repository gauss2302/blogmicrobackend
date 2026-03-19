import { z } from "zod";

export const loginSchema = z.object({
  email: z.email().trim(),
  password: z.string().min(8, "Password must be at least 8 characters."),
});

export const registerSchema = z.object({
  email: z.email().trim(),
  password: z.string().min(8, "Password must be at least 8 characters."),
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .max(100, "Name must be at most 100 characters."),
});

export const exchangeAuthCodeSchema = z.object({
  authCode: z.string().trim().min(1, "Auth code is required."),
  codeVerifier: z.string().trim().optional(),
});

export type LoginInput = z.infer<typeof loginSchema>;
export type RegisterInput = z.infer<typeof registerSchema>;
export type ExchangeAuthCodeInput = z.infer<typeof exchangeAuthCodeSchema>;
