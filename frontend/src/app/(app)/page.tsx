"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { api, ApiError, type UserOrder } from "@/lib/api";
import { formatDate, formatPoints, orderStatusLabels } from "@/lib/format";
import { markLoggedOut } from "@/lib/session";

export default function OrdersPage() {
  const router = useRouter();
  const [orders, setOrders] = useState<UserOrder[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [number, setNumber] = useState("");
  const [pending, setPending] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await api.orders();
      setOrders(data);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        markLoggedOut();
        router.replace("/login");
        return;
      }
      setError(err instanceof ApiError ? err.message : "Ошибка загрузки");
    }
  }, [router]);

  useEffect(() => {
    let active = true;
    api.orders().then(
      (data) => {
        if (!active) return;
        setOrders(data);
        setError(null);
      },
      (err) => {
        if (!active) return;
        if (err instanceof ApiError && err.status === 401) {
          markLoggedOut();
          router.replace("/login");
          return;
        }
        setError(err instanceof ApiError ? err.message : "Ошибка загрузки");
      },
    );
    return () => {
      active = false;
    };
  }, [router]);

  useEffect(() => {
    if (!orders?.some((o) => o.status === "NEW" || o.status === "PROCESSING")) {
      return;
    }
    const timer = setInterval(load, 5000);
    return () => clearInterval(timer);
  }, [orders, load]);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    const value = number.trim();
    if (!value) return;
    setPending(true);
    try {
      const status = await api.addOrder(value);
      if (status === 202) {
        toast.success("Заказ добавлен");
      } else {
        toast.info("Этот заказ уже загружен");
      }
      setNumber("");
      await load();
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        markLoggedOut();
        router.replace("/login");
        return;
      }
      toast.error(err instanceof ApiError ? err.message : "Ошибка");
    } finally {
      setPending(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle>Новый заказ</CardTitle>
          <CardDescription>
            Введите номер заказа — он будет отправлен на проверку
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="flex flex-col gap-3 sm:flex-row">
            <Input
              inputMode="numeric"
              placeholder="Номер заказа"
              value={number}
              onChange={(e) => setNumber(e.target.value)}
              className="font-mono"
            />
            <Button type="submit" disabled={pending || !number.trim()}>
              {pending ? "Отправка…" : "Добавить"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0">
          <div>
            <CardTitle>Мои заказы</CardTitle>
            <CardDescription>Статусы обновляются автоматически</CardDescription>
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Обновить"
            onClick={() => load()}
          >
            <RefreshCw className="size-4" />
          </Button>
        </CardHeader>
        <CardContent>
          {error ? (
            <Alert variant="destructive">
              <AlertTitle>Не удалось загрузить заказы</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : orders === null ? (
            <div className="flex flex-col gap-2">
              <div className="h-9 animate-pulse rounded-md bg-muted" />
              <div className="h-9 animate-pulse rounded-md bg-muted" />
              <div className="h-9 animate-pulse rounded-md bg-muted" />
            </div>
          ) : orders.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">
              Заказов пока нет — добавьте первый номер заказа
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Номер</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead className="text-right">Начисление</TableHead>
                  <TableHead className="text-right">Загружен</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.map((order) => (
                  <TableRow key={order.number}>
                    <TableCell className="font-mono">{order.number}</TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={orderStatusLabels[order.status]?.className}
                      >
                        {orderStatusLabels[order.status]?.label ?? order.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {order.accrual != null
                        ? `+${formatPoints(order.accrual)}`
                        : "—"}
                    </TableCell>
                    <TableCell className="text-right text-muted-foreground">
                      {formatDate(order.uploaded_at)}
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
