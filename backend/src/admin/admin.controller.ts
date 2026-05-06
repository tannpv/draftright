import {
  Controller, Get, Post, Patch, Delete, Body, Param, Query, UseGuards, Res, Header, BadRequestException,
} from '@nestjs/common';
import { Response } from 'express';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { Roles } from '../common/decorators/roles.decorator';
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';
import { RewriteLogService } from '../rewrite/rewrite-log.service';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { AppSettings } from './entities/app-settings.entity';
import { AdminUser } from './entities/admin-user.entity';
import { PaymentService } from '../payment/payment.service';
import * as bcrypt from 'bcryptjs';
import { GrantSubscriptionDto } from './dto/grant-subscription.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { ReleasesService } from '../updates/releases.service';
import { ErrorsService } from '../errors/errors.service';
import { FixProposalCron } from '../errors/fix-proposal.cron';

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
    private readonly paymentService: PaymentService,
    private readonly releasesService: ReleasesService,
    private readonly errorsService: ErrorsService,
    private readonly fixProposalCron: FixProposalCron,
  ) {}

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

  @Post('releases')
  async upsertRelease(@Body() body: {
    platform: string;
    version: string;
    download_url: string;
    release_notes?: string;
    required?: boolean;
  }) {
    if (!['mac', 'windows', 'linux', 'android', 'ios'].includes(body.platform)) {
      throw new BadRequestException(
        `platform must be one of: mac, windows, linux, android, ios`,
      );
    }
    if (!body.version || !body.download_url) {
      throw new BadRequestException('version and download_url are required');
    }
    return this.releasesService.upsert(body);
  }

  // --- Settings ---

  @Get('settings')
  async getSettings() {
    let settings = await this.settingsRepo.findOne({ where: {} });
    if (!settings) {
      settings = this.settingsRepo.create();
      await this.settingsRepo.save(settings);
    }
    return settings;
  }

  @Patch('settings')
  async updateSettings(@Body() body: Partial<AppSettings>) {
    let settings = await this.settingsRepo.findOne({ where: {} });
    if (!settings) {
      settings = this.settingsRepo.create();
      await this.settingsRepo.save(settings);
    }
    await this.settingsRepo.update(settings.id, body);
    return this.settingsRepo.findOne({ where: { id: settings.id } });
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
  async listUsers(@Query('search') search?: string, @Query('page') page?: string, @Query('limit') limit?: string) {
    const result = await this.usersService.findAll({
      search, page: page ? parseInt(page) : 1, limit: limit ? parseInt(limit) : 20,
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
    return { user, subscription: sub, usage_today: usageToday, recent_usage: recentUsage };
  }

  @Patch('users/:id')
  async updateUser(@Param('id') id: string, @Body() dto: UpdateUserDto) {
    return this.usersService.update(id, dto as any);
  }

  @Get('plans')
  async listPlans() { return this.plansService.findAll(); }

  @Post('plans')
  async createPlan(@Body() body: { name: string; daily_limit: number; price_cents: number; billing_period: string }) {
    return this.plansService.create(body as any);
  }

  @Patch('plans/:id')
  async updatePlan(@Param('id') id: string, @Body() body: Partial<{ name: string; daily_limit: number; price_cents: number; billing_period: string; is_active: boolean }>) {
    return this.plansService.update(id, body as any);
  }

  @Delete('plans/:id')
  async deletePlan(@Param('id') id: string) {
    await this.plansService.softDelete(id);
    return { success: true };
  }

  @Get('ai-providers')
  async listProviders() { return this.aiProvidersService.findAll(); }

  @Post('ai-providers')
  async createProvider(@Body() body: { name: string; type: string; endpoint_url: string; api_key?: string; model: string; temperature?: number }) {
    return this.aiProvidersService.create(body as any);
  }

  @Patch('ai-providers/:id')
  async updateProvider(@Param('id') id: string, @Body() body: Partial<{ name: string; type: string; endpoint_url: string; api_key: string; model: string; temperature: number; is_default: boolean; is_active: boolean }>) {
    return this.aiProvidersService.update(id, body as any);
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
  async listPayments(@Query('page') page?: string, @Query('limit') limit?: string, @Query('status') status?: string) {
    return this.paymentService.findAll({
      page: page ? parseInt(page) : 1,
      limit: limit ? parseInt(limit) : 20,
      status,
    });
  }

  @Post('payments/:id/confirm')
  async confirmPayment(@Param('id') id: string, @Body() body: { notes?: string }) {
    return this.paymentService.adminConfirm(id, body.notes);
  }

  // --- Admin Users CRUD ---

  @Get('admin-users')
  async listAdminUsers() {
    const users = await this.adminUserRepo.find({ order: { created_at: 'ASC' } });
    return users.map(({ password_hash, ...rest }) => rest);
  }

  @Post('admin-users')
  async createAdminUser(@Body() body: { email: string; password: string; name: string; role?: string }) {
    const existing = await this.adminUserRepo.findOne({ where: { email: body.email } });
    if (existing) throw new BadRequestException('Email already exists');

    const password_hash = await bcrypt.hash(body.password, 10);
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
    if (body.password) update.password_hash = await bcrypt.hash(body.password, 10);

    await this.adminUserRepo.update(id, update);
    const user = await this.adminUserRepo.findOneOrFail({ where: { id } });
    const { password_hash, ...result } = user;
    return result;
  }

  @Delete('admin-users/:id')
  async deleteAdminUser(@Param('id') id: string) {
    await this.adminUserRepo.update(id, { is_active: false });
    return { success: true };
  }
}
