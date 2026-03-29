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
exports.PlansService = void 0;
const common_1 = require("@nestjs/common");
const typeorm_1 = require("@nestjs/typeorm");
const typeorm_2 = require("typeorm");
const plan_entity_1 = require("./entities/plan.entity");
let PlansService = class PlansService {
    constructor(plansRepo) {
        this.plansRepo = plansRepo;
    }
    async findAll() {
        return this.plansRepo.find({ order: { created_at: 'ASC' } });
    }
    async findById(id) {
        return this.plansRepo.findOne({ where: { id } });
    }
    async findFreePlan() {
        const plan = await this.plansRepo.findOne({ where: { billing_period: plan_entity_1.BillingPeriod.NONE, is_active: true } });
        if (!plan)
            throw new Error('Free plan not found. Run seed first.');
        return plan;
    }
    async create(data) {
        const plan = this.plansRepo.create(data);
        return this.plansRepo.save(plan);
    }
    async update(id, data) {
        await this.plansRepo.update(id, data);
        return this.plansRepo.findOneOrFail({ where: { id } });
    }
    async softDelete(id) {
        await this.plansRepo.update(id, { is_active: false });
    }
};
exports.PlansService = PlansService;
exports.PlansService = PlansService = __decorate([
    (0, common_1.Injectable)(),
    __param(0, (0, typeorm_1.InjectRepository)(plan_entity_1.Plan)),
    __metadata("design:paramtypes", [typeorm_2.Repository])
], PlansService);
//# sourceMappingURL=plans.service.js.map