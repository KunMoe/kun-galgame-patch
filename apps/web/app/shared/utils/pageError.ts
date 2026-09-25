// Nitro prints a stack trace for every fatal error, and fatal 404s were 10,468
// of them in 12 hours of prod, burying real failures. The server renders the
// error page for a non-fatal throw anyway; only the client needs fatal to
// replace the page. The text rides `message`: `statusMessage` is the HTTP
// status line, where h3 strips the Chinese and warns on every request.
export const kunPageError = (statusCode: number, message: string) =>
  createError({
    statusCode,
    message,
    fatal: import.meta.client || statusCode >= 500
  })
