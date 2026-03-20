import { useEffect, useState } from "react";
import { fetchAvatar } from "@/lib/api";

interface AuthorAvatarProps {
  name: string;
  email: string;
  size?: number;
}

export function AuthorAvatar({ name, email, size = 24 }: AuthorAvatarProps) {
  const [avatarUrl, setAvatarUrl] = useState<string>("");

  useEffect(() => {
    if (!email) return;
    fetchAvatar(email).then(setAvatarUrl);
  }, [email]);

  const initials = name
    .split(/\s+/)
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);

  return (
    <span className="inline-flex items-center gap-1.5" title={`${name} <${email}>`}>
      {avatarUrl ? (
        <img
          src={avatarUrl}
          alt={name}
          width={size}
          height={size}
          className="rounded-full shrink-0"
          onError={(e) => {
            // Fall back to initials on load error
            e.currentTarget.style.display = "none";
            e.currentTarget.nextElementSibling?.classList.remove("hidden");
          }}
        />
      ) : null}
      <span
        className={`${avatarUrl ? "hidden" : ""} inline-flex items-center justify-center rounded-full bg-muted text-muted-foreground text-[10px] font-medium shrink-0`}
        style={{ width: size, height: size }}
      >
        {initials}
      </span>
      <span className="text-xs text-muted-foreground truncate max-w-[120px]">{name}</span>
    </span>
  );
}
