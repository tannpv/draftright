import {
  Body,
  Controller,
  HttpCode,
  Post,
  Req,
  UseGuards,
} from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { Request } from 'express';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { ExtractRequestDto, ExtractResponseDto } from './dto/extract.dto';
import { ExtractionService } from './extraction.service';

@ApiTags('extraction')
@Controller('extract')
export class ExtractionController {
  constructor(private readonly extractionService: ExtractionService) {}

  @UseGuards(JwtAuthGuard)
  @ApiBearerAuth()
  @HttpCode(200)
  @Post()
  async extract(@Req() _req: Request, @Body() dto: ExtractRequestDto): Promise<ExtractResponseDto> {
    return this.extractionService.extract(dto.text, dto.kinds);
  }
}
