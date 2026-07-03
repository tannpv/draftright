import { validate } from 'class-validator';
import { plainToInstance } from 'class-transformer';
import { RewriteDto } from './rewrite.dto';

describe('RewriteDto', () => {
  describe('input_kind validation (Go parity contract)', () => {
    it('rejects invalid input_kind with the exact parity message', async () => {
      const dto = plainToInstance(RewriteDto, {
        text: 'hi',
        tone: 'simple',
        input_kind: 'banana',
      });
      const errors = await validate(dto);
      const inputKindError = errors.find((e) => e.property === 'input_kind');
      expect(inputKindError?.constraints?.isIn).toBe(
        'input_kind must be one of the following values: typed, speech',
      );
    });

    it('accepts valid input_kind values', async () => {
      for (const kind of ['typed', 'speech']) {
        const dto = plainToInstance(RewriteDto, {
          text: 'hi',
          tone: 'simple',
          input_kind: kind,
        });
        const errors = await validate(dto);
        expect(errors).toEqual([]);
      }
    });

    it('allows input_kind to be omitted', async () => {
      const dto = plainToInstance(RewriteDto, {
        text: 'hi',
        tone: 'simple',
      });
      const errors = await validate(dto);
      expect(errors).toEqual([]);
    });
  });
});
