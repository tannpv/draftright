import { IsString, IsOptional, IsIn } from 'class-validator';
import { ApiProperty, ApiPropertyOptional } from '@nestjs/swagger';
import { TONE_IDS } from '../tones';

export class RewriteDto {
  @ApiProperty({ example: 'This is some text to rewrite' })
  @IsString()
  text: string;

  @ApiProperty({ example: 'polished', enum: TONE_IDS })
  @IsIn(TONE_IDS)
  tone: string;

  @ApiPropertyOptional({ example: 'Vietnamese' })
  @IsOptional()
  @IsString()
  target_language?: string;

  @ApiPropertyOptional({ example: 'English', description: 'Source language hint (auto-detected if not provided)' })
  @IsOptional()
  @IsString()
  source_language?: string;
}
