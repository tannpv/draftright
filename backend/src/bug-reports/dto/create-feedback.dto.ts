import { IsString, IsOptional, IsIn, MaxLength, MinLength } from 'class-validator';

export const FEEDBACK_KINDS = ['bug', 'feature'] as const;
export const TARGET_PLATFORMS = ['playground', 'mobile', 'windows', 'mac', 'linux'] as const;

export type FeedbackKind = (typeof FEEDBACK_KINDS)[number];
export type TargetPlatform = (typeof TARGET_PLATFORMS)[number];

/**
 * Body for `POST /feedback`. JSON (no file upload on this route — feature
 * requests don't take screenshots; the legacy `POST /bug-reports` route
 * still handles multipart bug reports with screenshots).
 */
export class CreateFeedbackDto {
  @IsIn(FEEDBACK_KINDS)
  kind: FeedbackKind;

  /** Required when kind === 'feature'; ignored for bugs. Validated in the service against length 1-80. */
  @IsOptional() @IsString() @MaxLength(80)
  title?: string;

  /** Required when kind === 'feature'; ignored for bugs. */
  @IsOptional() @IsIn(TARGET_PLATFORMS)
  target_platform?: TargetPlatform;

  @IsString() @MinLength(1) @MaxLength(2000)
  description: string;

  @IsString() @MaxLength(50)
  source: string;

  @IsOptional() @IsString() @MaxLength(50)
  app_version?: string;

  // See create-bug-report.dto for the rationale on widening from 100 → 255.
  @IsOptional() @IsString() @MaxLength(255)
  os_info?: string;

  @IsOptional() @IsString() @MaxLength(255)
  user_email?: string;
}
