import { IsString, IsOptional, MaxLength, MinLength, IsBoolean } from 'class-validator';

export class CreateBugReportDto {
  @IsString()
  @MinLength(1)
  description: string;

  @IsString()
  @MaxLength(50)
  source: string;

  @IsOptional() @IsString() @MaxLength(50)
  app_version?: string;

  // Cap matches the DB column. Android's Platform.operatingSystemVersion
  // includes the full kernel build string (often >150 chars on Xiaomi /
  // Samsung), so the previous @MaxLength(100) silently 400'd every Android
  // bug report. The service still slices to 100 before writing to the row,
  // so this widened cap is just a permissive ingress.
  @IsOptional() @IsString() @MaxLength(255)
  os_info?: string;

  @IsOptional() @IsString() @MaxLength(255)
  user_email?: string;

  /**
   * JSON-encoded string posted via multipart form. The service is
   * responsible for parsing it.
   */
  @IsOptional() @IsString()
  context?: string;

  /**
   * Honeypot. Legitimate clients never populate this — the form widget hides
   * it with display:none or a tabindex=-1 trick — so a non-empty value is a
   * reliable signal that an automated form-filler submitted the report. The
   * controller drops these silently with a 201 to deny the bot the failure
   * signal it would use to retry. Field name "website" is conventional bait.
   */
  @IsOptional() @IsString() @MaxLength(255)
  website?: string;
}

export class UpdateBugReportDto {
  @IsOptional() @IsString() @MaxLength(20)
  status?: string;

  @IsOptional() @IsString()
  admin_notes?: string;

  /** Feature requests: edit the title. */
  @IsOptional() @IsString() @MaxLength(80)
  title?: string;

  /** Feature requests: re-classify the target platform. */
  @IsOptional() @IsString() @MaxLength(20)
  target_platform?: string;

  /** Feature requests: hide/show on the public board. */
  @IsOptional() @IsBoolean()
  is_public?: boolean;
}
