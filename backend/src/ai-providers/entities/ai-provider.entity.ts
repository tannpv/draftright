import {
  Entity, PrimaryGeneratedColumn, Column, CreateDateColumn, UpdateDateColumn,
} from 'typeorm';

export enum AiProviderType {
  OPENAI = 'openai',
  OLLAMA = 'ollama',
  CUSTOM = 'custom',
}

@Entity('ai_providers')
export class AiProvider {
  @PrimaryGeneratedColumn('uuid')
  id: string;

  @Column({ type: 'varchar', length: 255 })
  name: string;

  @Column({ type: 'enum', enum: AiProviderType })
  type: AiProviderType;

  @Column({ type: 'varchar', length: 500 })
  endpoint_url: string;

  @Column({ type: 'varchar', length: 500, default: '' })
  api_key: string;

  @Column({ type: 'varchar', length: 100 })
  model: string;

  @Column({ type: 'decimal', precision: 3, scale: 2, default: 0.3 })
  temperature: number;

  @Column({ type: 'boolean', default: false })
  is_default: boolean;

  @Column({ type: 'boolean', default: true })
  is_active: boolean;

  @CreateDateColumn()
  created_at: Date;

  @UpdateDateColumn()
  updated_at: Date;
}
