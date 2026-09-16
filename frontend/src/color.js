// HSBK becomes display RGB only here. A square-root brightness curve follows
// Hikari's preview approach while keeping zero brightness visibly off.
export function rgb(c, power = 65535) {
 const h = ((c.Hue % 360) + 360) % 360 / 60, s = c.Saturation / 100;
 const v = Math.sqrt(Math.max(0, Math.min(1, c.Brightness / 100 * power / 65535)));
 const chroma = v * s, x = chroma * (1 - Math.abs(h % 2 - 1)), m = v - chroma;
 const pairs = [[chroma,x,0],[x,chroma,0],[0,chroma,x],[0,x,chroma],[x,0,chroma],[chroma,0,x]];
 const hsv = pairs[Math.floor(h)].map(a => (a + m) * 255);
 const t = Math.max(10, Math.min(400, c.Kelvin / 100));
 const clamp = a => Math.max(0,Math.min(255,a));
 const white = t<=66 ? [255,clamp(99.4708025861*Math.log(t)-161.1195681661),t<=19?0:clamp(138.5177312231*Math.log(t-10)-305.0447927307)] : [clamp(329.698727446*Math.pow(t-60,-.1332047592)),clamp(288.1221695283*Math.pow(t-60,-.0755148492)),255];
 return hsv.map((a,i)=>Math.round(a-m*255+(white[i]*.62+255*.38)*m));
}
export const cssColor = (c,p) => `rgb(${rgb(c,p).join(' ')})`;
