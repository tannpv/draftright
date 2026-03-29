import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { User } from './entities/user.entity';

@Injectable()
export class UsersService {
  constructor(
    @InjectRepository(User)
    private readonly usersRepo: Repository<User>,
  ) {}

  async findByEmail(email: string): Promise<User | null> {
    return this.usersRepo.findOne({ where: { email } });
  }

  async findById(id: string): Promise<User | null> {
    return this.usersRepo.findOne({ where: { id } });
  }

  async create(data: { email: string; password_hash: string; name: string; role?: string }): Promise<User> {
    const user = this.usersRepo.create(data);
    return this.usersRepo.save(user);
  }

  async update(id: string, data: Partial<User>): Promise<User> {
    await this.usersRepo.update(id, data);
    return this.usersRepo.findOneOrFail({ where: { id } });
  }

  async count(): Promise<number> {
    return this.usersRepo.count();
  }

  async findAll(options: { search?: string; page?: number; limit?: number }): Promise<{ users: User[]; total: number }> {
    const { search, page = 1, limit = 20 } = options;
    const qb = this.usersRepo.createQueryBuilder('user');

    if (search) {
      qb.where('user.email ILIKE :search OR user.name ILIKE :search', { search: `%${search}%` });
    }

    qb.orderBy('user.created_at', 'DESC')
      .skip((page - 1) * limit)
      .take(limit);

    const [users, total] = await qb.getManyAndCount();
    return { users, total };
  }
}
