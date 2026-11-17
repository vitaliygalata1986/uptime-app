import { AuthForm } from "@/components/auth/form";
/** Exposes login to anonymous users through the auth form's session guard. */
export default function LoginPage() { return <AuthForm mode="login" />; }
