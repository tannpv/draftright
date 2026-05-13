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

  @IsOptional() @IsString() @MaxLength(100)
  os_info?: string;

  @IsOptional() @IsString() @MaxLength(255)
  user_email?: string;

  /**
   * JSON-encoded string posted via multipart form. The service is
   * responsible for parsing it.
   */
  @IsOptional() @IsString()
  context?: string;
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
