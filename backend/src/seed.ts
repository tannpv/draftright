import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { UsersService } from './users/users.service';
import { PlansService } from './plans/plans.service';
import { SubscriptionsService } from './subscriptions/subscriptions.service';
import { AiProvidersService } from './ai-providers/ai-providers.service';
import { AiProviderType } from './ai-providers/entities/ai-provider.entity';
import { BillingPeriod } from './plans/entities/plan.entity';
import { UserRole } from './users/entities/user.entity';
import * as bcrypt from 'bcrypt';

async function seed() {
  const app = await NestFactory.createApplicationContext(AppModule);

  const usersService = app.get(UsersService);
  const plansService = app.get(PlansService);
  const subscriptionsService = app.get(SubscriptionsService);
  const aiProvidersService = app.get(AiProvidersService);

  // 1. Create Free plan if not exists
  const existingPlans = await plansService.findAll();
  let freePlan = existingPlans.find(p => p.billing_period === BillingPeriod.NONE);
  if (!freePlan) {
    freePlan = await plansService.create({
      name: 'Free',
      daily_limit: 10,
      price_cents: 0,
      billing_period: BillingPeriod.NONE,
      is_active: true,
    });
    console.log('Created Free plan');
  } else {
    console.log('Free plan already exists');
  }

  // 2. Create admin user if not exists
  const adminEmail = process.env.ADMIN_EMAIL || 'admin@draftright.com';
  const adminPassword = process.env.ADMIN_PASSWORD || 'admin123';
  const existingAdmin = await usersService.findByEmail(adminEmail);
  if (!existingAdmin) {
    const password_hash = await bcrypt.hash(adminPassword, 10);
    const admin = await usersService.create({
      email: adminEmail,
      password_hash,
      name: 'Admin',
      role: UserRole.ADMIN,
    });
    await subscriptionsService.createFreeSubscription(admin.id, freePlan.id);
    console.log(`Created admin user: ${adminEmail}`);
  } else {
    console.log('Admin user already exists');
  }

  // 3. Create default AI provider if none exists
  const existingProviders = await aiProvidersService.findAll();
  if (existingProviders.length === 0) {
    await aiProvidersService.create({
      name: 'OpenAI',
      type: AiProviderType.OPENAI,
      endpoint_url: 'https://api.openai.com/v1/chat/completions',
      api_key: process.env.OPENAI_API_KEY || '',
      model: 'gpt-4o-mini',
      temperature: 0.3,
      is_default: true,
      is_active: true,
    });
    console.log('Created default OpenAI provider');
  } else {
    console.log('AI providers already exist');
  }

  await app.close();
  console.log('Seed complete');
}

seed().catch((err) => {
  console.error('Seed failed:', err);
  process.exit(1);
});
