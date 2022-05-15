/* eslint-disable @typescript-eslint/no-non-null-assertion */
import { nanoid } from 'nanoid';
import { ValidationError } from 'yup';
import { torrentMagnetValidator, torrentSlugValidator } from '../validators/torrent.validator';

test('test torrent slug validator', async () => {
  const slug = nanoid(5).toLowerCase();
  const isValid = await torrentSlugValidator.params!.validate({ slug });
  expect(isValid).toBeTruthy();
});

test('test torrent slug validator : should fail', async () => {
  const slug = 'hjasd7as';
  try {
    await torrentSlugValidator.params!.validate({ slug });
  } catch (error) {
    expect(error).toBeInstanceOf(ValidationError);
  }
});

test('test torrent magnet link validator', async () => {
  const magnet =
    'magnet:?xt=urn:btih:0b4b504797e5638095c44d43b5abdca580be8fa0&dn=%5BOhys-Raws%5D%20Tate%20no%20Yuusha%20no%20Nariagari%20Season%202%20-%2006%20%28AT-X%201280x720%20x264%20AAC%29.mp4&tr=http%3A%2F%2Fnyaa.tracker.wf%3A7777%2Fannounce&tr=udp%3A%2F%2Fopen.stealth.si%3A80%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&tr=udp%3A%2F%2Fexodus.desync.com%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.torrent.eu.org%3A451%2Fannounce';
  const isValid = await torrentMagnetValidator.body!.validate({ magnet });
  expect(isValid).toBeTruthy();
});

test('test torrent magnet link validator: should fail', async () => {
  const magnet =
    'magnet:?t=urn:btih:0b4b504797e5638095c44d43b5abdca580be8fa0&dn=%5BOhys-Raws%5D%20Tate%20no%20Yuusha%20no%20Nariagari%20Season%202%20-%2006%20%28AT-X%201280x720%20x264%20AAC%29.mp4&tr=http%3A%2F%2Fnyaa.tracker.wf%3A7777%2Fannounce&tr=udp%3A%2F%2Fopen.stealth.si%3A80%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce&tr=udp%3A%2F%2Fexodus.desync.com%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.torrent.eu.org%3A451%2Fannounce';
  try {
    await torrentMagnetValidator.body!.validate({ magnet });
  } catch (error) {
    expect(error).toBeInstanceOf(ValidationError);
  }
});
