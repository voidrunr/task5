"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { AuthForm } from "@/components/auth-form";
import { api } from "@/lib/api";
import { isLoggedIn, markLoggedIn } from "@/lib/session";

export default function LoginPage() {
  const router = useRouter();

  useEffect(() => {
    if (isLoggedIn()) {
      router.replace("/");
    }
  }, [router]);

  return (
    <AuthForm
      mode="login"
      title="Вход"
      description="Войдите в аккаунт Гофермарта"
      submitLabel="Войти"
      pendingLabel="Входим…"
      onSubmit={async (login, password) => {
        await api.login(login, password);
        markLoggedIn();
        router.replace("/");
      }}
      footer={
        <>
          Нет аккаунта?{" "}
          <Link href="/register" className="text-foreground underline-offset-4 hover:underline">
            Зарегистрироваться
          </Link>
        </>
      }
    />
  );
}
