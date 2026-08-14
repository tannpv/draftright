import { IsBoolean, IsOptional, IsString, MaxLength } from 'class-validator';
import { PROFILE_FIELD_MAX_LEN, STYLE_NOTES_MAX_LEN } from '../user-context.constants';

/**
 * Payload for PUT /me/context. Every field optional so a client can patch just
 * one; the service fills unset fields from the existing row (or defaults).
 * Length caps mirror the entity/constants — one source of truth.
 */
export class UpdateUserContextDto {
  @IsOptional()
  @IsBoolean()
  enabled?: boolean;

  @IsOptional()
  @IsString()
  @MaxLength(PROFILE_FIELD_MAX_LEN)
  job_title?: string;

  @IsOptional()
  @IsString()
  @MaxLength(PROFILE_FIELD_MAX_LEN)
  industry?: string;

  @IsOptional()
  @IsString()
  @MaxLength(PROFILE_FIELD_MAX_LEN)
  audience?: string;

  @IsOptional()
  @IsString()
  @MaxLength(STYLE_NOTES_MAX_LEN)
  style_notes?: string;
}
