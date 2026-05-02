import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import { ExtensionTokenService } from './extension-token.service';
import { JwtAuthGuard } from './jwt-auth.guard';

const REQUIRED_SCOPE = 'rewrite';

/**
 * Guard for /rewrite that accepts either a regular user JWT (via the
 * existing JwtAuthGuard) or a dr_ext_* extension token (via
 * ExtensionTokenService.validate).
 *
 * Extension tokens must carry the 'rewrite' scope. Other endpoints
 * keep using JwtAuthGuard directly so an extension token can never
 * reach /auth/me, /payment, etc.
 */
@Injectable()
export class RewriteAuthGuard implements CanActivate {
  constructor(
    private readonly extService: ExtensionTokenService,
    private readonly jwtGuard: JwtAuthGuard,
  ) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const req = context.switchToHttp().getRequest();
    const header = req.headers.authorization;
    if (!header || !header.startsWith('Bearer ')) {
      throw new UnauthorizedException('Missing bearer token');
    }
    const token = header.slice('Bearer '.length);

    if (token.startsWith('dr_ext_')) {
      const validated = await this.extService.validate(token);
      if (!validated) throw new UnauthorizedException('Invalid extension token');
      if (!validated.scopes.includes(REQUIRED_SCOPE)) {
        throw new UnauthorizedException('Token missing rewrite scope');
      }
      // Match the shape downstream code expects from JwtStrategy.validate.
      req.user = {
        id: validated.userId,
        email: '',
        role: '',
        isAdmin: false,
        via: 'extension_token',
        tokenId: validated.tokenId,
      };
      return true;
    }

    return (await this.jwtGuard.canActivate(context)) as boolean;
  }
}
