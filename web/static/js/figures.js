// Mercenary figure, shared by the tavern and the field. The body takes the
// nick colour; the class shows as a silhouette detail readable at a glance.
const { Graphics } = PIXI;

export const CLASSES = {
  ranger: { name: 'Следопыт', mark: 0x3f7d3a },
  mage: { name: 'Маг', mark: 0x6b3fbf },
  pyro: { name: 'Пиромант', mark: 0xd2452b },
  cleric: { name: 'Жрец', mark: 0xf2e6a6 },
  rogue: { name: 'Разбойник', mark: 0x2b2b33 },
  bard: { name: 'Бард', mark: 0xc8932f },
  warrior: { name: 'Воитель', mark: 0x9aa0a8 },
};

const INK = 0x000000;

// drawFigure draws a figure standing on (0, 0), about 34 px tall.
export function drawFigure(g, color, cls) {
  const mark = CLASSES[cls]?.mark ?? 0x777777;
  // Things behind the body.
  if (cls === 'warrior') g.circle(-10, -6, 8).fill(mark).stroke({ width: 2, color: INK }); // shield
  if (cls === 'ranger') g.arc(11, -10, 11, -1.3, 1.3).stroke({ width: 3, color: 0x6b4a2a }); // bow
  if (cls === 'bard') g.ellipse(-11, -4, 6, 8).fill(0x8a5a2a).stroke({ width: 2, color: INK }); // lute

  g.roundRect(-8, -14, 16, 16, 4).fill(color).stroke({ width: 2, color: INK }); // body
  g.circle(0, -22, 8).fill(0xe8c9a0).stroke({ width: 2, color: INK }); // head

  switch (cls) {
    case 'mage': // pointed hat
      g.poly([-10, -26, 0, -46, 10, -26]).fill(mark).stroke({ width: 2, color: INK });
      break;
    case 'pyro': // flame on a hood
      g.poly([-9, -25, 0, -34, 9, -25]).fill(mark).stroke({ width: 2, color: INK });
      g.poly([-3, -34, 0, -42, 3, -34]).fill(0xffb347);
      break;
    case 'cleric': // halo
      g.ellipse(0, -33, 9, 3).stroke({ width: 3, color: 0xffe680 });
      break;
    case 'rogue': // hood and mask
      g.arc(0, -22, 9, Math.PI, 0).fill(mark);
      g.rect(-7, -23, 14, 4).fill(mark);
      break;
    case 'bard': // feathered cap
      g.ellipse(0, -28, 10, 4).fill(mark).stroke({ width: 2, color: INK });
      g.poly([4, -30, 14, -42, 8, -29]).fill(0xe04a3a);
      break;
    case 'warrior': // horned helmet
      g.arc(0, -23, 9, Math.PI, 0).fill(mark).stroke({ width: 2, color: INK });
      g.poly([-9, -26, -14, -36, -6, -29]).fill(0xf2e6c9);
      g.poly([9, -26, 14, -36, 6, -29]).fill(0xf2e6c9);
      break;
    case 'ranger': // green hood
      g.arc(0, -23, 9, Math.PI, 0).fill(mark).stroke({ width: 2, color: INK });
      break;
  }
}

export function figure(color, cls) {
  const g = new Graphics();
  drawFigure(g, color, cls);
  return g;
}
