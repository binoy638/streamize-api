import Bcrypt from 'bcrypt';

export class PasswordManager {
  static async toHash(password: string): Promise<string> {
    const salt = await Bcrypt.genSalt(10);
    return Bcrypt.hash(password, salt);
  }

  static compare(storedPassword: string, suppliedPassword: string): Promise<boolean> {
    return Bcrypt.compare(storedPassword, suppliedPassword);
  }
}
