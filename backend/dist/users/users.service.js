"use strict";
var __decorate = (this && this.__decorate) || function (decorators, target, key, desc) {
    var c = arguments.length, r = c < 3 ? target : desc === null ? desc = Object.getOwnPropertyDescriptor(target, key) : desc, d;
    if (typeof Reflect === "object" && typeof Reflect.decorate === "function") r = Reflect.decorate(decorators, target, key, desc);
    else for (var i = decorators.length - 1; i >= 0; i--) if (d = decorators[i]) r = (c < 3 ? d(r) : c > 3 ? d(target, key, r) : d(target, key)) || r;
    return c > 3 && r && Object.defineProperty(target, key, r), r;
};
var __metadata = (this && this.__metadata) || function (k, v) {
    if (typeof Reflect === "object" && typeof Reflect.metadata === "function") return Reflect.metadata(k, v);
};
var __param = (this && this.__param) || function (paramIndex, decorator) {
    return function (target, key) { decorator(target, key, paramIndex); }
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.UsersService = void 0;
const common_1 = require("@nestjs/common");
const typeorm_1 = require("@nestjs/typeorm");
const typeorm_2 = require("typeorm");
const user_entity_1 = require("./entities/user.entity");
let UsersService = class UsersService {
    constructor(usersRepo) {
        this.usersRepo = usersRepo;
    }
    async findByEmail(email) {
        return this.usersRepo.findOne({ where: { email } });
    }
    async findById(id) {
        return this.usersRepo.findOne({ where: { id } });
    }
    async create(data) {
        const user = this.usersRepo.create(data);
        return this.usersRepo.save(user);
    }
    async update(id, data) {
        await this.usersRepo.update(id, data);
        return this.usersRepo.findOneOrFail({ where: { id } });
    }
    async count() {
        return this.usersRepo.count();
    }
    async findAll(options) {
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
};
exports.UsersService = UsersService;
exports.UsersService = UsersService = __decorate([
    (0, common_1.Injectable)(),
    __param(0, (0, typeorm_1.InjectRepository)(user_entity_1.User)),
    __metadata("design:paramtypes", [typeorm_2.Repository])
], UsersService);
//# sourceMappingURL=users.service.js.map