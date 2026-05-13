import {
  Controller, Post, Body, Req, UploadedFile, UseInterceptors,
  BadRequestException, HttpCode,
} from '@nestjs/common';
import { FileInterceptor } from '@nestjs/platform-express';
import { ApiTags, ApiOperation, ApiConsumes } from '@nestjs/swagger';
import { Request } from 'express';
import { BugReportsService } from './bug-reports.service';
import { CreateBugReportDto } from './dto/create-bug-report.dto';
import { decodeOptionalUserId } from './jwt-user';

const MAX_BYTES = 5 * 1024 * 1024; // 5 MB
const ALLOWED_MIMES = ['image/png', 'image/jpeg', 'image/jpg'];

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
  constructor(private readonly bugReports: BugReportsService) {}

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
