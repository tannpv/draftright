import { IsString, IsOptional, IsObject, MaxLength } from 'class-validator';

export class CreateErrorReportDto {
  @IsString()
  @MaxLength(20)
  platform: string;       // 'ios' | 'android' | 'macos' | 'windows' | 'linux' | 'web'

  @IsOptional() @IsString() @MaxLength(50)
  app_version?: string;

  @IsOptional() @IsString() @MaxLength(20)
  severity?: string;      // 'fatal' | 'error' | 'warning' | 'info' (default: 'error')

  @IsOptional() @IsString() @MaxLength(200)
  error_type?: string;    // exception class / error code

  @IsOptional() @IsString()
  message?: string;

  @IsOptional() @IsString()
  stack_trace?: string;

  @IsOptional() @IsObject()
  context?: Record<string, any>;

  @IsOptional() @IsString() @MaxLength(100)
  device_id?: string;

  /** Honeypot. See backend/src/bug-reports/dto/create-bug-report.dto.ts. */
  @IsOptional() @IsString() @MaxLength(255)
  website?: string;
}
