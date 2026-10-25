import type { OrderStatus } from "@/lib/api";

const pointsFormatter = new Intl.NumberFormat("ru-RU", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatPoints(value: number): string {
  return pointsFormatter.format(value);
}

export function formatDate(value: string | Date): string {
  const date = typeof value === "string" ? new Date(value) : value;
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString("ru-RU", { dateStyle: "short", timeStyle: "short" });
}

export const orderStatusLabels: Record<
  OrderStatus,
  { label: string; className: string }
> = {
  NEW: {
    label: "Новый",
    className: "bg-secondary text-secondary-foreground",
  },
  PROCESSING: {
    label: "В обработке",
    className: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300",
  },
  INVALID: {
    label: "Некорректный",
    className: "bg-destructive/10 text-destructive",
  },
  PROCESSED: {
    label: "Обработан",
    className: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300",
  },
};
