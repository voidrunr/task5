const KEY = "gofermart_logged_in";

export function isLoggedIn(): boolean {
  return window.localStorage.getItem(KEY) === "1";
}

export function markLoggedIn(): void {
  window.localStorage.setItem(KEY, "1");
}

export function markLoggedOut(): void {
  window.localStorage.setItem(KEY, "0");
}
