export type OrderStatus = "NEW" | "PROCESSING" | "INVALID" | "PROCESSED";

export interface UserOrder {
  number: string;
  status: OrderStatus;
  accrual?: number;
  uploaded_at: string;
}

export interface Balance {
  current: number;
  withdrawn: number;
}

export interface Withdrawal {
  order: string;
  sum: number;
  processed_at: string;
}

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

const statusMessages: Record<number, string> = {
  0: "Не удалось связаться с сервером",
  400: "Некорректный запрос",
  401: "Требуется войти в аккаунт",
  402: "Недостаточно средств на балансе",
  409: "Конфликт: такой ресурс уже существует",
  422: "Номер заказа не прошёл проверку по Луну",
  429: "Слишком много запросов, попробуйте позже",
  500: "Внутренняя ошибка сервера",
};

const errorMessages: Record<string, string> = {
  "login already taken": "Пользователь с таким логином уже существует",
  unauthorized: "Неверный логин или пароль",
  "invalid order number": "Номер заказа не прошёл проверку по Луну",
  "order already uploaded by another user":
    "Этот заказ уже добавлен другим пользователем",
  "not enough funds": "Недостаточно средств на балансе",
  "bad request": "Некорректный запрос",
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, init);
  } catch {
    throw new ApiError(0, statusMessages[0]);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  if (!res.ok) {
    const detail = (await res.text()).trim();
    throw new ApiError(res.status, translate(res.status, detail));
  }

  const contentType = res.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    return (await res.json()) as T;
  }

  return undefined as T;
}

function translate(status: number, detail: string): string {
  return (
    errorMessages[detail] ?? statusMessages[status] ?? statusMessages[500]
  );
}


function jsonBody(data: unknown): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  };
}

export const api = {
  async login(login: string, password: string): Promise<void> {
    await request<void>("/api/user/login", jsonBody({ login, password }));
  },

  async register(login: string, password: string): Promise<void> {
    await request<void>("/api/user/register", jsonBody({ login, password }));
  },

  async orders(): Promise<UserOrder[]> {
    const data = await request<UserOrder[] | undefined>("/api/user/orders");
    return data ?? [];
  },

  async addOrder(number: string): Promise<number> {
    let res: Response;
    try {
      res = await fetch("/api/user/orders", {
        method: "POST",
        headers: { "Content-Type": "text/plain" },
        body: number,
      });
    } catch {
      throw new ApiError(0, statusMessages[0]);
    }
    if (!res.ok) {
      const detail = (await res.text()).trim();
      throw new ApiError(res.status, translate(res.status, detail));
    }
    return res.status;
  },

  async balance(): Promise<Balance> {
    return request<Balance>("/api/user/balance");
  },

  async withdraw(order: string, sum: number): Promise<void> {
    await request<void>("/api/user/balance/withdraw", jsonBody({ order, sum }));
  },

  async withdrawals(): Promise<Withdrawal[]> {
    const data = await request<Withdrawal[] | undefined>(
      "/api/user/withdrawals",
    );
    return data ?? [];
  },
};
