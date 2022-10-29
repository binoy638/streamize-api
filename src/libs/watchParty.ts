/* eslint-disable max-classes-per-file */
/* eslint-disable unicorn/no-null */
export interface Member {
  id: string; // socket id
  name: string;
  isHost: boolean;
  isWatching: boolean; // keep track of whether the member is watching the stream
}

type PlayerStatus = 'playing' | 'paused' | 'stopped' | 'buffering';

export interface IPlayer {
  torrentID: string;
  slug: string;
  title: string;
  currentTime: number;
  duration: number;
  status: PlayerStatus;
}

// data required to create a watch party
export interface PartyConstructor {
  name: string;
  slug: string;
  maxViewers: number;
  partyPlayerControl: boolean;
  host: string;
}

export interface RoomInfo {
  slug: string;
  name: string;
  maxViewers: number;
  partyPlayerControl: boolean;
  host: string;
  members: Member[];
  playerInfo: IPlayer | null;
}

export class Player {
  private _torrentID: string;

  private _slug: string;

  private _title: string;

  private _currentTime: number;

  private _duration: number;

  private _status: PlayerStatus;

  constructor({ torrentID, slug, title, currentTime, duration, status }: IPlayer) {
    this._torrentID = torrentID;
    this._slug = slug;
    this._title = title;
    this._currentTime = currentTime;
    this._duration = duration;
    this._status = status;
  }

  get playerInfo(): IPlayer {
    return {
      torrentID: this._torrentID,
      slug: this._slug,
      title: this._title,
      currentTime: this._currentTime,
      duration: this._duration,
      status: this._status,
    };
  }

  set playerInfo(playerInfo: IPlayer) {
    this._torrentID = playerInfo.torrentID;
    this._slug = playerInfo.slug;
    this._title = playerInfo.title;
    this._currentTime = playerInfo.currentTime;
    this._duration = playerInfo.duration;
    this._status = playerInfo.status;
  }

  set torrentID(torrentID: string) {
    this._torrentID = torrentID;
  }

  get torrentID(): string {
    return this._torrentID;
  }

  set slug(slug: string) {
    this._slug = slug;
  }

  get slug(): string {
    return this._slug;
  }

  set title(title: string) {
    this._title = title;
  }

  get title(): string {
    return this._title;
  }

  set currentTime(currentTime: number) {
    this._currentTime = currentTime;
  }

  get currentTime(): number {
    return this._currentTime;
  }

  set duration(duration: number) {
    this._duration = duration;
  }

  get duration(): number {
    return this._duration;
  }

  set status(status: PlayerStatus) {
    this._status = status;
  }

  get status(): PlayerStatus {
    return this._status;
  }
}

class WatchParty {
  private _name: string;

  private _slug: string;

  private _maxViewers: number;

  private _partyPlayerControl: boolean;

  private _host: string;

  private _members: Member[];

  private _player: Player | null;

  constructor(party: PartyConstructor) {
    this._name = party.name;
    this._slug = party.slug;
    this._maxViewers = party.maxViewers;
    this._partyPlayerControl = party.partyPlayerControl;
    this._host = party.host;
    this._members = [];
    this._player = null;
  }

  public addMember(member: Member): void {
    this._members.push(member);
  }

  public removeMember(id: string): void {
    this._members = this._members.filter(m => m.id !== id);
  }

  public getMember(memberId: string): Member | undefined {
    return this._members.find(m => m.id === memberId);
  }

  get members(): Member[] {
    return this._members;
  }

  get slug(): string {
    return this._slug;
  }

  get roomInfo(): RoomInfo {
    return {
      name: this._name,
      slug: this._slug,
      maxViewers: this._maxViewers,
      partyPlayerControl: this._partyPlayerControl,
      host: this._host,
      members: this._members,
      playerInfo: this._player ? this._player.playerInfo : null,
    };
  }

  set playerControl(bool: boolean) {
    this._partyPlayerControl = bool;
  }

  get playerControl(): boolean {
    return this._partyPlayerControl;
  }

  public initPlayer(playerInfo: IPlayer | null): void {
    if (playerInfo) {
      this._player = new Player(playerInfo);
      return;
    }
    this._player = null;
  }

  public getPlayer(): Player | null {
    return this._player;
  }
}

export default WatchParty;
