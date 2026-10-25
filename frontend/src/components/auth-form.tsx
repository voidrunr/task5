"use client";

import { useState } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface AuthFormProps {
  mode: "login" | "register";
  title: string;
  description: string;
  submitLabel: string;
  pendingLabel: string;
  footer: React.ReactNode;
  onSubmit: (login: string, password: string) => Promise<void>;
}

export function AuthForm({
  mode,
  title,
  description,
  submitLabel,
  pendingLabel,
  footer,
  onSubmit,
}: AuthFormProps) {
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!login.trim() || !password) {
      setError("Заполните логин и пароль");
      return;
    }
    setPending(true);
    setError(null);
    try {
      await onSubmit(login.trim(), password);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка");
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="flex min-h-full flex-1 items-center justify-center px-4 py-12">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="login">Логин</Label>
              <Input
                id="login"
                autoComplete="username"
                value={login}
                onChange={(e) => setLogin(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password">Пароль</Label>
              <Input
                id="password"
                type="password"
                autoComplete={
                  mode === "login" ? "current-password" : "new-password"
                }
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error ? (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            <Button type="submit" disabled={pending}>
              {pending ? pendingLabel : submitLabel}
            </Button>
          </form>
          <div className="mt-4 text-center text-sm text-muted-foreground">
            {footer}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
