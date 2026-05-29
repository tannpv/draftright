import {
  Controller, Post, Body, Req, UploadedFile, UseInterceptors,
  BadRequestException, HttpCode, Logger,
} from '@nestjs/common';
import { FileInterceptor } from '@nestjs/platform-express';
import { ApiTags, ApiOperation, ApiConsumes } from '@nestjs/swagger';
import { Throttle } from '@nestjs/throttler';
import { Request } from 'express';
import { BugReportsService } from './bug-reports.service';
import { CreateBugReportDto } from './dto/create-bug-report.dto';
import { decodeOptionalUserId } from './jwt-user';

const MAX_BYTES = 5 * 1024 * 1024; // 5 MB
// Accept every modern phone-camera / screenshot format. Samsung + Pixel
// galleries hand back HEIC/HEIF; Pixel screenshots are PNG; iOS share-sheet
// can deliver WebP; older Androids hand back GIF for short screen recordings.
// Anything narrower silently 400s the submission for users with default
// camera settings (real failure mode observed 2026-05-29 on Galaxy A52).
const ALLOWED_MIMES = [
  'image/png',
  'image/jpeg', 'image/jpg',
  'image/webp',
  'image/heic', 'image/heif',
  'image/gif',
];

/**
 * Public endpoint that any client (admin portal, marketing site, web
 * playground, native apps, mobile keyboard/share extensions) POSTs to
 * when a user reports a bug. Auth is optional — a Bearer JWT in the
 * Authorization header gets stamped on user_id; anonymous reports are
 * accepted too so we don't lose feedback from logged-out users.
 */
@ApiTags('bug-reports')
@Controller('bug-reports')
export class BugReportsController {
  private readonly logger = new Logger(BugReportsController.name);
  constructor(private readonly bugReports: BugReportsService) {}

  // Anonymous endpoint → much tighter throttle than the global default.
  // 5 reports / minute / IP and 30 / hour / IP stops scripted form spam
  // while leaving plenty of headroom for legitimate users banging out a
  // run of reports during a bad session.
  @Throttle({
    minute: { limit: 5, ttl: 60_000 },
    hour:   { limit: 30, ttl: 3_600_000 },
  })
  @Post()
  @HttpCode(201)
  @ApiOperation({ summary: 'Submit a user-reported bug from any client' })
  @ApiConsumes('multipart/form-data')
  @UseInterceptors(
    FileInterceptor('screenshot', {
      limits: { fileSize: MAX_BYTES },
      fileFilter: (_req, file, cb) => {
        if (!ALLOWED_MIMES.includes(file.mimetype)) {
          cb(new BadRequestException('only PNG or JPEG screenshots are accepted'), false);
          return;
        }
        cb(null, true);
      },
    }),
  )
  async create(
    @Body() dto: CreateBugReportDto,
    @UploadedFile() file: any,
    @Req() req: Request,
  ) {
    // Honeypot: clients leave the `website` field empty; bots that scrape the
    // form fill every field they see. A filled honeypot quietly succeeds (so
    // the bot has no signal to retry with) but no row is written.
    if (dto.website && dto.website.trim().length > 0) {
      this.logger.warn(`Honeypot triggered (IP=${req.ip}, source=${dto.source}) — dropping submission`);
      return { id: null, status: 'received' };
    }

    const userId = decodeOptionalUserId(req);

    const row = await this.bugReports.create(
      dto,
      file
        ? {
            buffer: file.buffer,
            originalname: file.originalname,
            mimetype: file.mimetype,
            size: file.size,
          }
        : undefined,
      userId,
    );
    return { id: row.id, message: 'Bug report received. Thanks!' };
  }
}
