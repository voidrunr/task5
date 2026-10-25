"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { AuthForm } from "@/components/auth-form";
import { api } from "@/lib/api";
import { isLoggedIn, markLoggedIn } from "@/lib/session";

export default function RegisterPage() {
  const router = useRouter();

  useEffect(() => {
    if (isLoggedIn()) {
      router.replace("/");
    }
  }, [router]);

  return (
    <AuthForm
      mode="register"
      title="Регистрация"
      description="Создайте аккаунт Гофермарта"
      submitLabel="Зарегистрироваться"
      pendingLabel="Создаём…"
      onSubmit={async (login, password) => {
        await api.register(login, password);
        markLoggedIn();
        router.replace("/");
      }}
      footer={
        <>
          Уже есть аккаунт?{" "}
          <Link href="/login" className="text-foreground underline-offset-4 hover:underline">
            Войти
          </Link>
        </>
      }
    />
  );
}
