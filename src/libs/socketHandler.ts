import { Socket } from 'socket.io';
import logger from '../config/logger';
import { WatchPartyModel } from '../models/watchParty';
import WatchParty, { Member, PartyConstructor } from './watchParty';
// const onConnection = (socket: Socket): void => {
//   console.log('a user connected :', socket.id);
//   socket.on(SyncStreamsEvents.CREATED, (data: Stream) => {
//     socket.join(data.id);
//     syncStreams.addStream(data);
//   });
//   socket.on(SyncStreamsEvents.NEW_MEMBER_JOINED, (data: { streamId: string; member: string }) => {
//     socket.join(data.streamId);
//     syncStreams.addMember(data.streamId, data.member);
//   });
//   socket.on(SyncStreamsEvents.PLAY, (data: { streamId: string }) => {
//     console.log(data);
//     socket.to(data.streamId).emit(SyncStreamsEvents.PLAY, data);
//   });
//   socket.on(SyncStreamsEvents.PAUSE, (data: { streamId: string }) => {
//     socket.to(data.streamId).emit(SyncStreamsEvents.PAUSE);
//   });
//   socket.on(SyncStreamsEvents.SEEKED, (data: { streamId: string; seekTime: number }) => {
//     socket.to(data.streamId).emit(SyncStreamsEvents.SEEKED, data);
//   });
//   socket.on('disconnect', () => {
//     console.log('a user disconnected :', socket.id);
//   });
// };

enum RoomEvent {
  // emitted when a user tries to join a room
  JOIN = 'join',
  PLAY = 'play',
  PAUSE = 'pause',
  SEEKED = 'seeked',
  PLAYER_UPDATE = 'playerUpdate',

  // client events

  JOINED = 'joined', // emitted when a user joins a room successfully
  PLAYED = 'played',
  PLAYER_UPDATED = 'playerUpdated',
}

enum RoomErrorEvent {
  ROOM_NOT_FOUND = 'room-not-found',
}

// INFO: it contains all the watch parties
const ROOMS = new Map<string, WatchParty>();

interface JoinRoomData {
  slug: string;
  member: Omit<Member, 'id'>;
}

const socketHandler = (socket: Socket): void => {
  logger.debug(`a user connected : ${socket.id}`);

  socket.on(RoomEvent.JOIN, async (data: JoinRoomData) => {
    const { slug, member } = data;

    const roomData = await WatchPartyModel.findOne({ slug });

    if (!roomData) {
      socket.emit(RoomErrorEvent.ROOM_NOT_FOUND, { slug });
      socket.disconnect();
      return;
    }

    // INFO: if the room is already present in the map, then add the member to the room otherwise create a new room
    let room = ROOMS.get(slug);
    if (!room) {
      logger.error(`room not found: ${slug}`);
      logger.info(`creating a new room: ${slug}`);
      const roomConstructor: PartyConstructor = {
        slug,
        host: String(roomData.host),
        name: roomData.name,
        maxViewers: roomData.maxViewers,
        partyPlayerControl: roomData.partyPlayerControl,
      };
      room = new WatchParty(roomConstructor);
      ROOMS.set(slug, room);
    }
    room.addMember({ ...member, id: socket.id });
    socket.join(slug);

    const { roomInfo } = room;

    socket.to(slug).emit(RoomEvent.JOINED, roomInfo);
  });

  // socket.on(RoomEvent.PLAYER_UPDATE, (data: { slug: string; playerInfo: PlayerInfo }) => {
  //   const { slug, playerInfo } = data;
  //   const room = ROOMS.get(slug);
  //   if (!room) {
  //     logger.error(`room not found: ${slug}`);
  //     return;
  //   }
  //   room.playerInfo = playerInfo;
  //   socket.to(slug).emit(RoomEvent.PLAYER_UPDATED, playerInfo);
  // });

  // socket.on(RoomEvent.PLAY, (data: { slug: string }) => {
  //   const { slug } = data;
  //   const room = ROOMS.get(slug);
  //   if (!room) {
  //     logger.error(`room not found: ${slug}`);
  //     return;
  //   }

  //   const { playerInfo } = room;

  //   if (!playerInfo) {
  //     logger.error(`player info not found: ${slug}`);
  //     return;
  //   }

  //   if (playerInfo.isPlaying) {
  //     logger.error(`player is already playing: ${slug}`);
  //     return;
  //   }

  //   room.isPlaying = true;

  //   socket.to(slug).emit(RoomEvent.PLAYED);
  // });

  socket.on('disconnect', () => {
    logger.debug(`a user disconnected : ${socket.id}`);
  });
};

export default socketHandler;
