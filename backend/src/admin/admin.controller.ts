import {
  Controller, Get, Post, Patch, Delete, Body, Param, Query, UseGuards, Res, Header, Request, BadRequestException, NotFoundException,
} from '@nestjs/common';
import { Response } from 'express';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { Roles } from '../common/decorators/roles.decorator';
import { UsersService } from '../users/users.service';
import { stripUserSecrets } from '../users/sanitize-user.util';
import { PlansService } from '../plans/plans.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import { maskProvider } from '../ai-providers/mask-provider.util';
import { containsMaskMarker } from '../common/mask-secret.util';
import { maskSettings, stripMaskedSecretsFromBody } from './mask-settings.util';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';
import { RewriteLogService } from '../rewrite/rewrite-log.service';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AppSettings } from './entities/app-settings.entity';
import { EmailLog } from '../email/entities/email-log.entity';
import { EmailTemplate } from '../email/entities/email-template.entity';
import { EMAIL_TEMPLATES, EMAIL_TEMPLATE_MAP } from '../email/email-templates';
import { AdminUser } from './entities/admin-user.entity';
import { PaymentService } from '../payment/payment.service';
import { hashPassword } from '../common/password-hash.util';
import { GrantSubscriptionDto } from './dto/grant-subscription.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { ReleasesService } from '../updates/releases.service';
import { PoliciesService } from '../updates/policies.service';
import { ErrorsService } from '../errors/errors.service';
import { FixProposalCron } from '../errors/fix-proposal.cron';
import { EmailService } from '../email/email.service';
import { parseListQuery, applyListQuery } from '../common/list-query';
import { BugReportsService } from '../bug-reports/bug-reports.service';
import { createReadStream } from 'fs';
import { promises as fsp } from 'fs';

@ApiTags('admin')
@ApiBearerAuth()
@UseGuards(JwtAuthGuard, RolesGuard)
@Roles('admin')
@Controller('admin')
export class AdminController {
  constructor(
    private readonly usersService: UsersService,
    private readonly plansService: PlansService,
    private readonly aiProvidersService: AiProvidersService,
    private readonly subscriptionsService: SubscriptionsService,
    private readonly usageService: UsageService,
    private readonly rewriteLogService: RewriteLogService,
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
    @InjectRepository(AdminUser)
    private readonly adminUserRepo: Repository<AdminUser>,
    @InjectRepository(EmailLog)
    private readonly emailLogRepo: Repository<EmailLog>,
    @InjectRepository(EmailTemplate)
    private readonly emailTemplateRepo: Repository<EmailTemplate>,
    private readonly paymentService: PaymentService,
    private readonly releasesService: ReleasesService,
    private readonly policiesService: PoliciesService,
    private readonly errorsService: ErrorsService,
    private readonly fixProposalCron: FixProposalCron,
    private readonly emailService: EmailService,
    private readonly bugReportsService: BugReportsService,
  ) {}

  // --- Bug reports (user-submitted feedback from any client) ---

  @Get('bug-reports')
  async listBugReports(@Query() q: Record<string, unknown>) {
    const query = parseListQuery(q);
    const status = typeof q.status === 'string' ? q.status : undefined;
    const kind = typeof q.kind === 'string' ? q.kind : undefined;
    const target_platform = typeof q.target_platform === 'string' ? q.target_platform : undefined;
    return this.bugReportsService.findAllPaginated({
      ...query,
      status: status as any,
      kind,
      target_platform,
    });
  }

  @Get('bug-reports/:id')
  async getBugReport(@Param('id') id: string) {
    return this.bugReportsService.findById(id);
  }

  @Get('bug-reports/:id/screenshot')
  async getBugReportScreenshot(
    @Param('id') id: string,
    @Res() res: Response,
  ) {
    const result = await this.bugReportsService.getScreenshotPath(id);
    if (!result) {
      throw new BadRequestException('no screenshot for this report');
    }
    try {
      await fsp.access(result.path);
    } catch {
      throw new BadRequestException('screenshot file missing on disk');
    }
    const ext = result.path.toLowerCase().endsWith('.png') ? 'image/png' : 'image/jpeg';
    res.setHeader('Content-Type', ext);
    res.setHeader(
      'Content-Disposition',
      `inline; filename="${result.filename.replace(/[^\w.\-]/g, '_')}"`,
    );
    createReadStream(result.path).pipe(res);
  }

  @Patch('bug-reports/:id')
  async updateBugReport(
    @Param('id') id: string,
    @Body() body: { status?: string; admin_notes?: string; title?: string; target_platform?: string; is_public?: boolean },
  ) {
    return this.bugReportsService.update(id, body);
  }

  @Delete('bug-reports/:id')
  async deleteBugReport(@Param('id') id: string) {
    await this.bugReportsService.delete(id);
    return { success: true };
  }

  /**
   * Manually trigger an AI fix-proposal for one bug. Useful when you
   * want analysis right now instead of waiting for the hourly cron.
   */
  @Post('bug-reports/:id/fix-proposal')
  async suggestBugFix(@Param('id') id: string) {
    return this.bugReportsService.suggestFix(id);
  }

  /**
   * Unified inbox — merges error_reports + bug_reports into one
   * time-sorted feed. Powers the admin /inbox page.
   *
   * Query params:
   *   ?kind=error|bug   filter to one source
   *   ?status=open      'open' = not resolved (status<4 for errors, status not in {resolved, wont_fix} for bugs)
   *   ?limit=50         default 50, max 100
   */
  /**
   * Lightweight counts for the admin top-bar inbox badge — kept separate
   * from /inbox (which paginates rows) so it can be polled every 60 s
   * without dragging the full payload.
   *
   * Returns counts of items in their initial "new" state, broken down by
   * type so the UI can render per-source badges if desired.
   */
  @Get('inbox/counts')
  async inboxCounts() {
    const [bugs, features, errors] = await Promise.all([
      this.bugReportsService.findAllPaginated({
        page: 1, limit: 1, status: 'new' as any, kind: 'bug',
        sort_by: 'created_at', sort_order: 'DESC',
      } as any),
      this.bugReportsService.findAllPaginated({
        page: 1, limit: 1, status: 'new' as any, kind: 'feature',
        sort_by: 'created_at', sort_order: 'DESC',
      } as any),
      this.errorsService.list({ status: 0, limit: 1, offset: 0 }),
    ]);
    const new_bugs = (bugs as any).total ?? 0;
    const new_features = (features as any).total ?? 0;
    const new_errors = (errors as any).total ?? 0;
    return {
      new_bugs,
      new_features,
      new_errors,
      total: new_bugs + new_features + new_errors,
    };
  }

  @Get('inbox')
  async listInbox(
    @Query('kind') kind?: string,
    @Query('status') status?: string,
    @Query('limit') limitStr?: string,
  ) {
    const limit = Math.min(parseInt(limitStr || '50', 10) || 50, 100);
    const wantErrors = kind !== 'bug';
    const wantBugs = kind !== 'error';
    const openOnly = status === 'open';

    const errorRows = wantErrors
      ? await this.errorsService.list({
          status: openOnly ? 0 : undefined,
          limit,
          offset: 0,
        })
      : { items: [], total: 0 };

    const bugRows = wantBugs
      ? await this.bugReportsService.findAllPaginated({
          page: 1,
          limit,
          status: openOnly ? 'new' : undefined,
          sort_by: 'created_at',
          sort_order: 'DESC',
        } as any)
      : { rows: [], total: 0 };

    type InboxItem = {
      kind: 'error' | 'bug';
      id: string;
      title: string;
      platform: string | null;
      app_version: string | null;
      status: string;
      created_at: Date;
      ai_fix_proposal: string | null;
      // Kind-specific extras (so the UI can deep-link without another fetch)
      error_type?: string | null;
      severity?: string | null;
      occurrence_count?: number;
      user_email?: string | null;
      has_screenshot?: boolean;
    };

    const errorItems: InboxItem[] = (errorRows.items as any[]).map(e => ({
      kind: 'error',
      id: e.id,
      title: [e.error_type, e.message].filter(Boolean).join(': ').slice(0, 200) || '(no message)',
      platform: e.platform ?? null,
      app_version: e.app_version ?? null,
      status: this.errorStatusLabel(e.status),
      created_at: e.last_seen_at ?? e.first_seen_at ?? e.created_at,
      ai_fix_proposal: e.ai_fix_proposal ?? null,
      error_type: e.error_type ?? null,
      severity: e.severity ?? null,
      occurrence_count: e.count ?? 0,
    }));

    const bugItems: InboxItem[] = (bugRows.rows as any[]).map(b => ({
      kind: 'bug',
      id: b.id,
      title: (b.description ?? '').slice(0, 200) || '(no description)',
      platform: b.os_info ?? null,
      app_version: b.app_version ?? null,
      status: b.status,
      created_at: b.created_at,
      ai_fix_proposal: b.ai_fix_proposal ?? null,
      user_email: b.user_email ?? null,
      has_screenshot: !!b.screenshot_path,
    }));

    const merged = [...errorItems, ...bugItems]
      .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      .slice(0, limit);

    return {
      items: merged,
      counts: {
        errors: errorRows.total,
        bugs: bugRows.total,
        returned: merged.length,
      },
    };
  }

  private errorStatusLabel(s: number): string {
    // Mirrors the int-enum used by error_reports.status
    return ['new', 'reviewing', 'resolved', 'fix_proposed', 'resolved', 'wont_fix'][s] ?? 'new';
  }

  // --- Error reports (Sentry-equivalent — collects bugs from all clients) ---

  @Get('errors')
  async listErrors(
    @Query('platform') platform?: string,
    @Query('status') status?: string,
    @Query('severity') severity?: string,
    @Query('limit') limit?: string,
    @Query('offset') offset?: string,
  ) {
    return this.errorsService.list({
      platform,
      status: status !== undefined ? Number(status) : undefined,
      severity,
      limit: limit ? Number(limit) : undefined,
      offset: offset ? Number(offset) : undefined,
    });
  }

  @Get('errors/:id')
  async getError(@Param('id') id: string) {
    const row = await this.errorsService.getOne(id);
    if (!row) throw new BadRequestException('not found');
    return row;
  }

  @Patch('errors/:id')
  async updateErrorStatus(
    @Param('id') id: string,
    @Body() body: { status: number; resolved_by?: string },
  ) {
    if (body.status === undefined) {
      throw new BadRequestException('status required');
    }
    return this.errorsService.setStatus(id, body.status, body.resolved_by);
  }

  @Delete('errors/:id')
  async deleteErrorReport(@Param('id') id: string) {
    return this.errorsService.deleteOne(id);
  }

  @Post('errors/:id/suggest-fix')
  async suggestFix(@Param('id') id: string) {
    return this.errorsService.suggestFix(id);
  }

  @Post('errors/run-ai-cron')
  async runFixProposalCron() {
    // Manual trigger for the hourly cron — useful for testing or
    // burning through a backlog without waiting for the schedule.
    await this.fixProposalCron.run();
    return { ok: true };
  }

  // --- App releases (Mac/Windows/Linux/Android/iOS update channel) ---

  @Get('releases')
  async listReleases() {
    return this.releasesService.listAll();
  }

  /**
   * Backwards-compat: existing release-publish.sh script POSTs without
   * a `channel` field. Defaults to 'direct'.
   */
  @Post('releases')
  async upsertRelease(@Body() body: {
    platform: string;
    channel?: string;
    version: string;
    download_url: string;
    sha256?: string;
    release_notes?: string;
    required?: boolean;
    enabled?: boolean;
  }) {
    return this.releasesService.upsertChannel({
      ...body,
      channel: body.channel ?? 'direct',
    });
  }

  @Delete('releases/:platform/:channel')
  async deleteRelease(
    @Param('platform') platform: string,
    @Param('channel') channel: string,
  ) {
    await this.releasesService.deleteChannel(platform, channel);
    return { ok: true };
  }

  @Post('release-policies')
  async upsertPolicy(@Body() body: {
    platform: string;
    preferred?: string;
    store_status?: string;
    notes?: string;
  }) {
    return this.policiesService.upsert(body);
  }

  // --- Settings ---

  @Get('email-logs')
  async emailLogs(@Query() q: Record<string, string>) {
    const limit = Math.min(parseInt(q.limit) || 50, 200);
    const page = Math.max(parseInt(q.page) || 1, 1);
    const where = q.status ? { status: q.status as any } : {};
    const [rows, total] = await this.emailLogRepo.findAndCount({
      where,
      order: { created_at: 'DESC' },
      take: limit,
      skip: (page - 1) * limit,
    });
    return { rows, total };
  }

  // --- Email templates (editable; falls back to built-in defaults) ---

  @Get('email-templates')
  async listEmailTemplates() {
    const overrides = await this.emailTemplateRepo.find();
    const byKey = new Map(overrides.map((o) => [o.template_key, o]));
    return EMAIL_TEMPLATES.map((def) => {
      const o = byKey.get(def.key);
      return {
        key: def.key,
        label: def.label,
        variables: def.variables,
        subject: o?.subject ?? def.subject,
        html: o?.html ?? def.html,
        customized: !!o,
        default_subject: def.subject,
        default_html: def.html,
      };
    });
  }

  @Patch('email-templates/:key')
  async updateEmailTemplate(@Param('key') key: string, @Body() body: { subject: string; html: string }) {
    if (!EMAIL_TEMPLATE_MAP[key]) throw new NotFoundException('Unknown template');
    await this.emailTemplateRepo.save(
      this.emailTemplateRepo.create({ template_key: key, subject: body.subject, html: body.html }),
    );
    return { ok: true };
  }

  /** Reset to the built-in default (remove the override row). */
  @Delete('email-templates/:key')
  async resetEmailTemplate(@Param('key') key: string) {
    await this.emailTemplateRepo.delete({ template_key: key });
    return { ok: true };
  }

  /** Rendered preview with sample data. */
  @Get('email-templates/:key/preview')
  async previewEmailTemplate(@Param('key') key: string) {
    if (!EMAIL_TEMPLATE_MAP[key]) throw new NotFoundException('Unknown template');
    const sample: Record<string, string> = {
      name: 'Tan', code: '123456', plan: 'Pro', amount: '124.000 ₫',
      expires: new Date(Date.now() + 30 * 86400000).toDateString(),
    };
    return this.emailService.renderTemplate(key, sample);
  }

  @Get('settings')
  async getSettings() {
    let settings = await this.settingsRepo.findOne({ where: {} });
    if (!settings) {
      settings = this.settingsRepo.create();
      await this.settingsRepo.save(settings);
    }
    return maskSettings(settings);
  }

  @Patch('settings')
  async updateSettings(@Body() body: Partial<AppSettings>) {
    // #30: drop masked secret echoes so a portal re-save can't overwrite a
    // stored key with its mask.
    stripMaskedSecretsFromBody(body);
    // Reject enabling a payment method that has no backend strategy (e.g.
    // paypal/momo) — otherwise the storefront advertises a tile that 400s
    // at checkout.
    if (body.payment_methods_enabled !== undefined) {
      this.paymentService.assertMethodsRegisterable(body.payment_methods_enabled);
    }
    let settings = await this.settingsRepo.findOne({ where: {} });
    if (!settings) {
      settings = this.settingsRepo.create();
      await this.settingsRepo.save(settings);
    }
    await this.settingsRepo.update(settings.id, body);
    return maskSettings(await this.settingsRepo.findOne({ where: { id: settings.id } }));
  }

  @Post('settings/test-email')
  async sendTestEmail(@Body() body: { to: string }) {
    if (!body?.to || !body.to.includes('@')) {
      throw new Error('Valid recipient email required');
    }
    await this.emailService.sendTestEmail(body.to);
    return { sent: true, to: body.to };
  }

  @Get('stats')
  async getStats() {
    const [total_users, active_subscriptions, rewrites_today, rewrites_this_month] = await Promise.all([
      this.usersService.count(),
      this.subscriptionsService.countActive(),
      this.usageService.countToday(),
      this.usageService.countThisMonth(),
    ]);
    return { total_users, active_subscriptions, rewrites_today, rewrites_this_month };
  }

  @Get('users')
  async listUsers(
    @Query('search') search?: string,
    @Query('page') page?: string,
    @Query('limit') limit?: string,
    @Query('status') status?: string,
    @Query('sort_by') sort_by?: string,
    @Query('sort_order') sort_order?: string,
  ) {
    const result = await this.usersService.findAll({
      search,
      page: page ? parseInt(page) : 1,
      limit: limit ? parseInt(limit) : 20,
      status: (status === 'active' || status === 'inactive' || status === 'all') ? status : undefined,
      sort_by,
      sort_order: sort_order?.toUpperCase() === 'ASC' ? 'ASC' : 'DESC',
    });
    const usersWithSubs = await Promise.all(
      result.users.map(async (user) => {
        const sub = await this.subscriptionsService.findActiveByUserId(user.id);
        const usageToday = await this.usageService.countTodayByUser(user.id);
        return {
          id: user.id, email: user.email, name: user.name, role: user.role,
          is_active: user.is_active, plan: sub?.plan?.name || 'None',
          usage_today: usageToday, created_at: user.created_at,
        };
      }),
    );
    return { users: usersWithSubs, total: result.total };
  }

  @Get('users/:id')
  async getUser(@Param('id') id: string) {
    const user = await this.usersService.findById(id);
    const sub = await this.subscriptionsService.findActiveByUserId(id);
    const usageToday = await this.usageService.countTodayByUser(id);
    const recentUsage = await this.usageService.findRecentByUser(id);
    return { user: stripUserSecrets(user), subscription: sub, usage_today: usageToday, recent_usage: recentUsage };
  }

  @Patch('users/:id')
  async updateUser(@Param('id') id: string, @Body() dto: UpdateUserDto) {
    return stripUserSecrets(await this.usersService.update(id, dto as any));
  }

  @Get('plans')
  async listPlans(@Query() q: Record<string, unknown>) {
    // If no query params provided, fall back to legacy unpaginated response (used elsewhere e.g. UserDetailPage).
    if (!q || (q.page === undefined && q.search === undefined && q.status === undefined && q.sort_by === undefined)) {
      return this.plansService.findAll();
    }
    return this.plansService.findAllPaginated(parseListQuery(q));
  }

  @Post('plans')
  async createPlan(@Body() body: { name: string; daily_limit: number; price_cents: number; billing_period: string; currency?: string; trial_days?: number; stripe_price_id?: string }) {
    return this.plansService.create(body as any);
  }

  @Patch('plans/:id')
  async updatePlan(@Param('id') id: string, @Body() body: Partial<{ name: string; daily_limit: number; price_cents: number; billing_period: string; is_active: boolean; currency: string; trial_days: number; stripe_price_id: string }>) {
    return this.plansService.update(id, body as any);
  }

  @Delete('plans/:id')
  async deletePlan(@Param('id') id: string) {
    await this.plansService.softDelete(id);
    return { success: true };
  }

  @Get('ai-providers/paginated')
  async listProvidersPaginated(@Query() q: Record<string, unknown>) {
    const res = await this.aiProvidersService.findAllPaginated(parseListQuery(q));
    return { ...res, rows: res.rows.map(maskProvider) };
  }

  @Get('ai-providers')
  async listProviders() { return (await this.aiProvidersService.findAll()).map(maskProvider); }

  @Post('ai-providers')
  async createProvider(@Body() body: { name: string; type: string; endpoint_url: string; api_key?: string; model: string; temperature?: number }) {
    if (containsMaskMarker(body.api_key)) delete body.api_key;
    return maskProvider(await this.aiProvidersService.create(body as any));
  }

  @Patch('ai-providers/:id')
  async updateProvider(@Param('id') id: string, @Body() body: Partial<{ name: string; type: string; endpoint_url: string; api_key: string; model: string; temperature: number; is_default: boolean; is_active: boolean }>) {
    if (containsMaskMarker(body.api_key)) delete body.api_key;
    return maskProvider(await this.aiProvidersService.update(id, body as any));
  }

  @Delete('ai-providers/:id')
  async deleteProvider(@Param('id') id: string) {
    await this.aiProvidersService.softDelete(id);
    return { success: true };
  }

  @Post('ai-providers/:id/test')
  async testProvider(@Param('id') id: string) {
    const provider = await this.aiProvidersService.findById(id);
    if (!provider) return { success: false, error: 'Provider not found' };
    try {
      const result = await this.aiProvidersService.callProvider(provider, 'Rewrite this text to be more concise.', 'This is a test sentence to verify the connection works properly.');
      return { success: true, response: result.text, response_time_ms: result.responseTimeMs };
    } catch (error: any) {
      return { success: false, error: error.message };
    }
  }

  @Post('subscriptions/grant')
  async grantSubscription(@Body() dto: GrantSubscriptionDto) {
    const expiresAt = dto.expires_at ? new Date(dto.expires_at) : undefined;
    return this.subscriptionsService.grant(dto.user_id, dto.plan_id, expiresAt);
  }

  @Get('analytics')
  async getAnalytics() {
    const breakdown = await this.subscriptionsService.getPlansBreakdown();
    const monthlyStats = await this.subscriptionsService.getMonthlyStats(12);

    // Calculate MRR: monthly subs * price + yearly subs * price/12
    let mrr = 0;
    const plansAll = await this.plansService.findAll();
    for (const b of breakdown) {
      const plan = plansAll.find(p => p.name === b.plan_name);
      if (plan && plan.price_cents > 0) {
        if (plan.billing_period === 'monthly') {
          mrr += b.active_count * plan.price_cents;
        } else if (plan.billing_period === 'yearly') {
          mrr += b.active_count * Math.round(plan.price_cents / 12);
        }
      }
    }

    const total_revenue = monthlyStats.reduce((sum, m) => sum + m.revenue_cents, 0);

    return {
      mrr,
      total_revenue,
      plans_breakdown: breakdown,
      monthly_stats: monthlyStats,
    };
  }

  @Get('transactions')
  async listTransactions(@Query('search') search?: string, @Query('page') page?: string, @Query('limit') limit?: string) {
    const result = await this.subscriptionsService.findAllPaginated({
      search,
      page: page ? parseInt(page) : 1,
      limit: limit ? parseInt(limit) : 20,
    });

    const transactions = result.subscriptions.map(sub => ({
      id: sub.id,
      user_email: sub.user?.email || '—',
      user_name: sub.user?.name || '—',
      user_id: sub.user_id,
      plan_name: sub.plan?.name || '—',
      price_cents: sub.plan?.price_cents || 0,
      store_type: sub.store_type,
      store_transaction_id: sub.store_transaction_id,
      status: sub.status,
      started_at: sub.started_at,
      expires_at: sub.expires_at,
      created_at: sub.created_at,
    }));

    return { transactions, total: result.total };
  }

  // --- Training Data (Fine-tuning) ---

  @Get('training-data/stats')
  async trainingDataStats() {
    const [total, byQuality] = await Promise.all([
      this.rewriteLogService.count(),
      this.rewriteLogService.countByQuality(),
    ]);
    return { total, ...byQuality };
  }

  @Get('training-data')
  async listTrainingData(@Query('page') page?: string, @Query('limit') limit?: string) {
    return this.rewriteLogService.findPending(
      page ? parseInt(page) : 1,
      limit ? parseInt(limit) : 20,
    );
  }

  @Patch('training-data/:id')
  async reviewTrainingData(@Param('id') id: string, @Body() body: { quality: 'approved' | 'rejected' }) {
    await this.rewriteLogService.updateQuality(id, body.quality);
    return { success: true };
  }

  @Get('training-data/export')
  async exportTrainingData(@Res() res: Response) {
    const jsonl = await this.rewriteLogService.exportApproved('jsonl');
    res.setHeader('Content-Type', 'application/jsonl');
    res.setHeader('Content-Disposition', 'attachment; filename=draftright-training-data.jsonl');
    res.send(jsonl);
  }

  // --- Payments ---

  @Get('payments/stats')
  async paymentStats() {
    return this.paymentService.getStats();
  }

  @Get('payments')
  async listPayments(
    @Query('page') page?: string,
    @Query('limit') limit?: string,
    @Query('status') status?: string,
    @Query('search') search?: string,
    @Query('sort_by') sort_by?: string,
    @Query('sort_order') sort_order?: string,
  ) {
    const { payments, total } = await this.paymentService.findAll({
      page: page ? parseInt(page) : 1,
      limit: limit ? parseInt(limit) : 20,
      status,
      search,
      sort_by,
      sort_order: sort_order?.toUpperCase() === 'ASC' ? 'ASC' : 'DESC',
    });
    // #48: the leftJoinAndSelect'd user is the raw entity (no
    // ClassSerializerInterceptor), so drop the six secret columns from each
    // nested user before it reaches an admin client. Mirrored byte-for-byte by
    // the Go port (StrippedUserDetail on the payment row's nested user).
    return {
      payments: payments.map((p) => ({ ...p, user: stripUserSecrets(p.user) })),
      total,
    };
  }

  @Post('payments/:id/confirm')
  async confirmPayment(@Param('id') id: string, @Body() body: { notes?: string }) {
    return this.paymentService.adminConfirm(id, body.notes);
  }

  @Post('payments/:id/refund')
  async refundPayment(@Param('id') id: string, @Body() body: { reason?: string }) {
    return this.paymentService.refund(id, body.reason);
  }

  // --- Admin Users CRUD ---

  @Get('admin-users')
  async listAdminUsers(@Query() q: Record<string, unknown>) {
    const query = parseListQuery(q);
    // Legacy callers that send no query params still get the full list shape.
    if (q?.page === undefined && q?.search === undefined && q?.status === undefined && q?.sort_by === undefined) {
      const users = await this.adminUserRepo.find({ order: { created_at: 'ASC' } });
      return users.map(({ password_hash, ...rest }) => rest);
    }
    const qb = this.adminUserRepo.createQueryBuilder('admin');
    const { rows, total } = await applyListQuery(
      qb,
      query,
      ['admin.name', 'admin.email', 'admin.role'],
      {
        name: 'admin.name',
        email: 'admin.email',
        role: 'admin.role',
        is_active: 'admin.is_active',
        created_at: 'admin.created_at',
      },
      'admin.created_at',
      'admin.is_active',
    );
    return { rows: rows.map(({ password_hash, ...rest }) => rest), total };
  }

  @Post('admin-users')
  async createAdminUser(@Body() body: { email: string; password: string; name: string; role?: string }) {
    const existing = await this.adminUserRepo.findOne({ where: { email: body.email } });
    if (existing) throw new BadRequestException('Email already exists');

    const password_hash = await hashPassword(body.password);
    const user = this.adminUserRepo.create({
      email: body.email,
      password_hash,
      name: body.name,
      role: body.role || 'admin',
    });
    const saved = await this.adminUserRepo.save(user);
    const { password_hash: _, ...result } = saved;
    return result;
  }

  @Patch('admin-users/:id')
  async updateAdminUser(@Param('id') id: string, @Body() body: { name?: string; email?: string; role?: string; is_active?: boolean; password?: string }) {
    const update: any = {};
    if (body.name !== undefined) update.name = body.name;
    if (body.email !== undefined) update.email = body.email;
    if (body.role !== undefined) update.role = body.role;
    if (body.is_active !== undefined) update.is_active = body.is_active;
    if (body.password) update.password_hash = await hashPassword(body.password);

    await this.adminUserRepo.update(id, update);
    const user = await this.adminUserRepo.findOneOrFail({ where: { id } });
    const { password_hash, ...result } = user;
    return result;
  }

  @Delete('admin-users/:id')
  async deleteAdminUser(@Param('id') id: string, @Request() req: any) {
    // #32: an admin must not deactivate their own account or the last active
    // admin (lockout / privilege-loss). Both → 400.
    if (id === req.user.id) {
      throw new BadRequestException('You cannot deactivate your own admin account');
    }
    const target = await this.adminUserRepo.findOne({ where: { id } });
    if (target && target.is_active) {
      const activeCount = await this.adminUserRepo.count({ where: { is_active: true } });
      if (activeCount <= 1) {
        throw new BadRequestException('Cannot deactivate the last active admin');
      }
    }
    await this.adminUserRepo.update(id, { is_active: false });
    return { success: true };
  }
}
