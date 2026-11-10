import Image from "next/image";
import { avatarURL, type User } from "@/lib/auth/api";

export function Avatar({ user, size, preview }: { user: User; size: number; preview?: string }) {
  const source = preview || (user.avatar_url ? avatarURL(user.avatar_url) : "");
  const style = { width: size, height: size, borderRadius: "50%", flexShrink: 0 };
  return source ? <Image src={source} unoptimized width={size} height={size} alt="Аватар пользователя" style={{ ...style, objectFit: "cover" }} /> :
    <span aria-label="Аватар пользователя" role="img" style={{ ...style, display: "grid", placeItems: "center", background: "#edf4ef", color: "var(--accent)", fontWeight: 600, fontSize: size / 2.5 }}>{Array.from(user.name || user.email)[0]?.toUpperCase()}</span>;
}
