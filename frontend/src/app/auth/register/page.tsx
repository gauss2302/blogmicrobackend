import { AuthShell } from "@/components/auth/auth-shell";
import { RegisterForm } from "@/components/auth/register-form";

export default function RegisterPage() {
  return (
    <AuthShell
      title="Create Account"
      subtitle="Create an account with email/password or continue securely with Google."
    >
      <RegisterForm />
    </AuthShell>
  );
}
