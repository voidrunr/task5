"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { api, ApiError, type Balance, type Withdrawal } from "@/lib/api";
import { formatDate, formatPoints } from "@/lib/format";
import { markLoggedOut } from "@/lib/session";

export default function BalancePage() {
  const router = useRouter();
  const [balance, setBalance] = useState<Balance | null>(null);
  const [withdrawals, setWithdrawals] = useState<Withdrawal[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [order, setOrder] = useState("");
  const [sum, setSum] = useState("");
  const [pending, setPending] = useState(false);

  const handleAuthError = useCallback(
    (err: unknown) => {
      if (err instanceof ApiError && err.status === 401) {
        markLoggedOut();
        router.replace("/login");
        return true;
      }
      return false;
    },
    [router],
  );

  const load = useCallback(async () => {
    try {
      const [b, w] = await Promise.all([api.balance(), api.withdrawals()]);
      setBalance(b);
      setWithdrawals(w);
      setError(null);
    } catch (err) {
      if (handleAuthError(err)) return;
      setError(err instanceof ApiError ? err.message : "Ошибка загрузки");
    }
  }, [handleAuthError]);

  useEffect(() => {
    let active = true;
    Promise.all([api.balance(), api.withdrawals()]).then(
      ([b, w]) => {
        if (!active) return;
        setBalance(b);
        setWithdrawals(w);
        setError(null);
      },
      (err) => {
        if (!active) return;
        if (handleAuthError(err)) return;
        setError(err instanceof ApiError ? err.message : "Ошибка загрузки");
      },
    );
    return () => {
      active = false;
    };
  }, [handleAuthError]);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    const value = Number(sum);
    if (!order.trim() || !sum.trim() || Number.isNaN(value)) return;
    setPending(true);
    try {
      await api.withdraw(order.trim(), value);
      toast.success("Средства списаны");
      setOrder("");
      setSum("");
      await load();
    } catch (err) {
      if (handleAuthError(err)) return;
      toast.error(err instanceof ApiError ? err.message : "Ошибка");
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader>
            <CardDescription>Текущий баланс</CardDescription>
            <CardTitle className="text-3xl tabular-nums">
              {balance ? `${formatPoints(balance.current)} баллов` : <Skeleton className="h-8 w-40" />}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>Списано</CardDescription>
            <CardTitle className="text-3xl tabular-nums">
              {balance ? `${formatPoints(balance.withdrawn)} баллов` : <Skeleton className="h-8 w-40" />}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      {error ? (
        <Alert variant="destructive">
          <AlertTitle>Не удалось загрузить данные</AlertTitle>
          <AlertDescription>
            {error}
            <Button variant="outline" size="sm" className="ml-3" onClick={() => load()}>
              Повторить
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Вывод баллов</CardTitle>
          <CardDescription>
            Укажите номер заказа и сумму списания
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="flex flex-col gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-2">
                <Label htmlFor="withdraw-order">Номер заказа</Label>
                <Input
                  id="withdraw-order"
                  inputMode="numeric"
                  placeholder="Номер заказа"
                  value={order}
                  onChange={(e) => setOrder(e.target.value)}
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="withdraw-sum">Сумма</Label>
                <Input
                  id="withdraw-sum"
                  type="number"
                  min="0.01"
                  step="0.01"
                  placeholder="0.00"
                  value={sum}
                  onChange={(e) => setSum(e.target.value)}
                />
              </div>
            </div>
            <Button
              type="submit"
              className="sm:self-start"
              disabled={pending || !order.trim() || !sum.trim()}
            >
              {pending ? "Списание…" : "Списать"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>История списаний</CardTitle>
        </CardHeader>
        <CardContent>
          {withdrawals === null ? (
            <div className="flex flex-col gap-2">
              <div className="h-9 animate-pulse rounded-md bg-muted" />
              <div className="h-9 animate-pulse rounded-md bg-muted" />
            </div>
          ) : withdrawals.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              Списаний пока не было
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Заказ</TableHead>
                  <TableHead className="text-right">Сумма</TableHead>
                  <TableHead className="text-right">Дата</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {withdrawals.map((w) => (
                  <TableRow key={`${w.order}-${w.processed_at}`}>
                    <TableCell className="font-mono">{w.order}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      −{formatPoints(w.sum)}
                    </TableCell>
                    <TableCell className="text-right text-muted-foreground">
                      {formatDate(w.processed_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
