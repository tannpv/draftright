import { Column, Entity, PrimaryColumn, UpdateDateColumn } from 'typeorm';

/**
 * Admin override for a built-in email template (keyed by EmailTemplateDef.key).
 * A row exists only when the admin has customized that template; otherwise the
 * default from email-templates.ts is used.
 */
@Entity('email_templates')
export class EmailTemplate {
  @PrimaryColumn({ type: 'varchar', length: 64 })
  template_key: string;

  @Column({ type: 'varchar', length: 255 })
  subject: string;

  @Column({ type: 'text' })
  html: string;

  @UpdateDateColumn()
  updated_at: Date;
}
