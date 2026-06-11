import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { PlansService } from './plans/plans.service';
import { AiProvidersService } from './ai-providers/ai-providers.service';
import { AiProviderType } from './ai-providers/entities/ai-provider.entity';
import { BillingPeriod } from './plans/entities/plan.entity';
import { AdminUser } from './admin/entities/admin-user.entity';
import { DataSource } from 'typeorm';
import { hashPassword } from './common/password-hash.util';

async function seed() {
  const app = await NestFactory.createApplicationContext(AppModule);

  const dataSource = app.get(DataSource);
  const plansService = app.get(PlansService);
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

  // 2. Create admin user in admin_users table if not exists
  const adminEmail = process.env.ADMIN_EMAIL || 'admin@draftright.com';
  const adminPassword = process.env.ADMIN_PASSWORD || 'admin123';
  const adminUserRepo = dataSource.getRepository(AdminUser);
  const existingAdmin = await adminUserRepo.findOne({ where: { email: adminEmail } });
  if (!existingAdmin) {
    const password_hash = await hashPassword(adminPassword);
    const admin = adminUserRepo.create({
      email: adminEmail,
      password_hash,
      name: 'Admin',
      role: 'admin',
    });
    await adminUserRepo.save(admin);
    console.log(`Created admin user in admin_users: ${adminEmail}`);
  } else {
    console.log('Admin user already exists in admin_users');
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
