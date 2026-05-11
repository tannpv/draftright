import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { User, UserRole, AuthProvider } from './entities/user.entity';

@Injectable()
export class UsersService {
  constructor(
    @InjectRepository(User)
    private readonly usersRepo: Repository<User>,
  ) {}

  async findByEmail(email: string): Promise<User | null> {
    return this.usersRepo
      .createQueryBuilder('u')
      .where('LOWER(u.email) = LOWER(:email)', { email: email.trim() })
      .getOne();
  }

  async findById(id: string): Promise<User | null> {
    return this.usersRepo.findOne({ where: { id } });
  }

  async create(data: Partial<User> & { email: string; name: string }): Promise<User> {
    const user = this.usersRepo.create(data);
    return this.usersRepo.save(user);
  }

  async findBySocialId(provider: AuthProvider, socialId: string): Promise<User | null> {
    const column = `${provider}_id`;
    return this.usersRepo.findOne({ where: { [column]: socialId } });
  }

  async update(id: string, data: Partial<User>): Promise<User> {
    await this.usersRepo.update(id, data);
    return this.usersRepo.findOneOrFail({ where: { id } });
  }

  async count(): Promise<number> {
    return this.usersRepo.count();
  }

  async findAll(options: {
    search?: string;
    page?: number;
    limit?: number;
    status?: 'all' | 'active' | 'inactive';
    sort_by?: string;
    sort_order?: 'ASC' | 'DESC';
  }): Promise<{ users: User[]; total: number }> {
    const { search, page = 1, limit = 20, status, sort_by, sort_order } = options;
    const qb = this.usersRepo.createQueryBuilder('user');

    if (search) {
      qb.andWhere('(user.email ILIKE :search OR user.name ILIKE :search)', { search: `%${search}%` });
    }
    if (status && status !== 'all') {
      qb.andWhere('user.is_active = :is_active', { is_active: status === 'active' });
    }

    const sortMap: Record<string, string> = {
      email: 'user.email',
      name: 'user.name',
      role: 'user.role',
      is_active: 'user.is_active',
      created_at: 'user.created_at',
    };
    const sortField = (sort_by && sortMap[sort_by]) || 'user.created_at';
    qb.orderBy(sortField, sort_order === 'ASC' ? 'ASC' : 'DESC')
      .skip((page - 1) * limit)
      .take(limit);

    const [users, total] = await qb.getManyAndCount();
    return { users, total };
  }
}
