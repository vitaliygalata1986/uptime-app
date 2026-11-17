import { AuthForm } from "@/components/auth/form";
/** Exposes registration and password confirmation through the auth form's session guard. */
export default function RegisterPage() { return <AuthForm mode="register" />; }
