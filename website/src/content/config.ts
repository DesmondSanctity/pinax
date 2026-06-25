import { defineCollection, z } from 'astro:content';

const docs = defineCollection({
  type: 'content',
  schema: z.object({
    title: z.string(),
    description: z.string().optional(),
    section: z.enum(['guides', 'reference', 'clients']).default('guides'),
    order: z.number().int().nonnegative().default(0),
  }),
});

export const collections = { docs };
