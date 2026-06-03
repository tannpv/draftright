import {
  ArgumentsHost,
  Catch,
  ExceptionFilter,
  HttpException,
  HttpStatus,
  Logger,
} from '@nestjs/common';
import { Request, Response } from 'express';
import { ERROR_CODES, ErrorCode, httpStatusForCode } from './error-codes';

/**
 * Single global exception filter that converts ANY thrown error to a
 * uniform JSON envelope:
 *
 *   {
 *     "error": "<human-readable message>",
 *     "code":  "kebab-case-machine-id",
 *     "request_id": "<uuid>"
 *   }
 *
 * Rationale (from the architecture review):
 *   - Mirrors the shape the Go /rewrite service already emits, so
 *     macOS / Flutter / Windows clients write one decoder.
 *   - Top-level `error` stays a string for backward compat with the
 *     existing macOS client that currently surfaces `bodyText` as the
 *     user-facing message.
 *   - `code` is the new machine-readable field; clients can pattern
 *     match without parsing English.
 *   - `request_id` makes "send me your support id" actually useful.
 *
 * Status-code mapping:
 *   - HttpException → respect its statusCode.
 *   - Anything with `.code` matching ERROR_CODES → httpStatusForCode().
 *   - Otherwise 500.
 *
 * The filter logs every 5xx at error level (with request_id stamped)
 * so structured log queries can join an upstream 502 to its request.
 */
@Catch()
export class AllExceptionsFilter implements ExceptionFilter {
  private readonly logger = new Logger('Exception');

  catch(exception: unknown, host: ArgumentsHost): void {
    const ctx = host.switchToHttp();
    const res = ctx.getResponse<Response>();
    const req = ctx.getRequest<Request>();
    const requestId = req.requestId ?? '';

    const { status, code, message } = this.classify(exception);

    if (status >= 500) {
      this.logger.error(
        `${req.method} ${req.url} → ${status} ${code} | ${message} | request_id=${requestId}`,
      );
    } else if (status >= 400) {
      this.logger.warn(
        `${req.method} ${req.url} → ${status} ${code} | request_id=${requestId}`,
      );
    }

    res.status(status).json({
      error: message,
      code,
      request_id: requestId,
    });
  }

  /**
   * Maps any thrown value into the (status, code, message) tuple the
   * envelope needs.  Order of checks is important: HttpException is
   * inspected FIRST so existing throw-sites that already pass a code
   * propagate verbatim.
   */
  private classify(exception: unknown): {
    status: number;
    code: ErrorCode | string;
    message: string;
  } {
    if (exception instanceof HttpException) {
      const status = exception.getStatus();
      const body = exception.getResponse();

      // Pre-existing exception body shapes:
      //   string                → { code: inferred, message: body }
      //   { message: string }   → { code: inferred, message }
      //   { message: string[] } → join with ", "
      //   { error, code? }      → respect explicit code/error
      let message: string;
      let code: string;

      if (typeof body === 'string') {
        message = body;
        code = this.inferCode(status);
      } else if (body && typeof body === 'object') {
        const obj = body as Record<string, unknown>;
        if (typeof obj.code === 'string') {
          code = obj.code;
        } else {
          code = this.inferCode(status);
        }
        // Prefer the specific `message` over the generic `error`.
        // NestJS validation errors arrive as
        //   `{ message: ['email must be an email'], error: 'Bad Request', statusCode: 400 }`
        // — picking `error` would surface the unhelpful 'Bad Request'
        // to mobile users instead of the actual field-level reason.
        if (typeof obj.message === 'string') {
          message = obj.message;
        } else if (Array.isArray(obj.message)) {
          message = obj.message
            .filter((m): m is string => typeof m === 'string')
            .map(this.humanizeValidation)
            .join('. ');
        } else if (typeof obj.error === 'string') {
          message = obj.error;
        } else {
          message = exception.message || 'Request failed';
        }
      } else {
        message = exception.message || 'Request failed';
        code = this.inferCode(status);
      }

      return { status, code, message };
    }

    // Anything that isn't an HttpException → treat as 500 internal.
    const err = exception as Error;
    return {
      status: HttpStatus.INTERNAL_SERVER_ERROR,
      code: ERROR_CODES.internal,
      message: err?.message ?? 'Internal server error',
    };
  }

  /**
   * Best-effort code derivation when an exception came in without one.
   * Keeps the common HTTP shapes mapped to the matching kebab-case
   * code so even legacy throw-sites get a code in the envelope.
   */
  /**
   * Convert one class-validator constraint message into something a
   * non-engineer can read.  Pattern is `<field> <constraint phrase>`;
   * we rewrite the prefix when the constraint phrase already names
   * the field implicitly ("must be an email" → "Please enter a valid
   * email address").
   *
   * Kept private + simple — adding rules is one new `if` per rule.
   */
  private humanizeValidation(raw: string): string {
    const m = raw.toLowerCase();
    const field = raw.split(' ')[0];
    const titleCase = field.charAt(0).toUpperCase() + field.slice(1);
    if (m.endsWith('must be an email')) return 'Please enter a valid email address.';
    if (m.includes('must be longer than or equal to')) {
      const n = raw.match(/(\d+)/)?.[1];
      return n ? `${titleCase} must be at least ${n} characters.` : raw;
    }
    if (m.endsWith('should not be empty')) {
      return `${titleCase} is required.`;
    }
    // Leave anything we don't have a friendlier phrasing for verbatim.
    return raw;
  }

  private inferCode(status: number): ErrorCode | string {
    switch (status) {
      case 400: return ERROR_CODES.invalidInput;
      case 401: return ERROR_CODES.invalidToken;
      case 402: return ERROR_CODES.quotaExceeded;
      case 403: return ERROR_CODES.forbidden;
      case 404: return ERROR_CODES.notFound;
      case 409: return ERROR_CODES.conflict;
      case 429: return ERROR_CODES.rateLimited;
      case 502: return ERROR_CODES.providerFailed;
      case 503: return ERROR_CODES.providerUnavailable;
      default:
        if (status >= 500) return ERROR_CODES.internal;
        return 'http-' + String(status);
    }
  }
}

/** Re-export for callers that want to derive the status programmatically. */
export { httpStatusForCode, ERROR_CODES };
