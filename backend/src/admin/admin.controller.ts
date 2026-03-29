import {
  Controller, Get, Post, Patch, Delete, Body, Param, Query, UseGuards,
} from '@nestjs/common';
import { ApiBearerAuth, ApiTags } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { RolesGuard } from '../common/guards/roles.guard';
import { Roles } from '../common/decorators/roles.decorator';
import { UsersService } from '../users/users.service';
import { PlansService } from '../plans/plans.service';
import { AiProvidersService } from '../ai-providers/ai-providers.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { UsageService } from '../usage/usage.service';
import { GrantSubscriptionDto } from './dto/grant-subscription.dto';
import { UpdateUserDto } from './dto/update-user.dto';

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
  ) {}

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
}
