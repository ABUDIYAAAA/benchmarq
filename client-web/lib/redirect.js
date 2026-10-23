// Post-login destinations come from the URL (?redirect=...). Only paths
// inside this app are honoured, so a crafted link cannot send a freshly
// signed-in user to another site.
export function safeRedirect(target, fallback = "/dashboard") {
  if (typeof target !== "string" || !target.startsWith("/") || target.startsWith("//") || target.startsWith("/\\")) {
    return fallback;
  }
  return target;
}
